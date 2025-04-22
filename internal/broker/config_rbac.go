package server

import (
	"encoding/json"
	"fmt"
	"strings"

	internal "github.com/jr200/nats-iam-broker/internal"
	"gopkg.in/yaml.v3"

	"github.com/nats-io/jwt/v2"
	"github.com/rs/zerolog/log"
)

// RoleBindingStrategy defines the strategy for matching role bindings.
type RoleBindingStrategy string

const (
	// StrategyBestMatch selects the binding with the most matching criteria.
	StrategyBestMatch RoleBindingStrategy = "best_match"
	// StrategyStrict requires all match criteria in a binding to succeed.
	StrategyStrict RoleBindingStrategy = "strict"
)

// UnmarshalYAML implements the yaml.Unmarshaler interface for RoleBindingStrategy.
func (rbs *RoleBindingStrategy) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}

	strLower := strings.ToLower(str)
	switch strLower {
	case string(StrategyStrict), string(StrategyBestMatch):
		*rbs = RoleBindingStrategy(strLower)
		return nil
	default:
		// Default to best_match if the value is empty or unrecognized
		if strLower == "" {
			log.Warn().Msgf("role_binding_matching_strategy is empty, defaulting to '%s'", StrategyBestMatch)
			*rbs = StrategyBestMatch
			return nil
		}
		log.Warn().Msgf("unrecognized role_binding_matching_strategy '%s', defaulting to '%s'", str, StrategyBestMatch)
		*rbs = StrategyBestMatch
		return nil // Or return an error if you want to reject invalid values: fmt.Errorf("invalid role binding strategy: %s", str)
	}
}

// Struct definitions
type Rbac struct {
	Accounts                    []UserAccountInfo   `yaml:"user_accounts"`
	RoleBinding                 []RoleBinding       `yaml:"role_binding"`
	Roles                       []Role              `yaml:"roles"`
	TokenMaxExpiry              Duration            `yaml:"token_max_expiration"`
	RoleBindingMatchingStrategy RoleBindingStrategy `yaml:"role_binding_matching_strategy"`
}

type UserAccountInfo struct {
	Name        string `yaml:"name"`
	PublicKey   string `yaml:"public_key"`
	SigningNKey NKey   `yaml:"signing_nkey"`
}

type RoleBinding struct {
	Account        string   `yaml:"user_account"`
	Roles          []string `yaml:"roles"`
	TokenMaxExpiry Duration `yaml:"token_max_expiration"`
	Match          []Match  `yaml:"match"`
}

type Match struct {
	Claim      string `yaml:"claim,omitempty"`
	Value      string `yaml:"value,omitempty"`
	Permission string `yaml:"permission,omitempty"`
}

type Role struct {
	Name        string      `yaml:"name"`
	Permissions Permissions `yaml:"permissions"`
	Limits      Limits      `yaml:"limits"`
}

type Permissions struct {
	Pub  jwt.Permission     `yaml:"pub,omitempty"`
	Sub  jwt.Permission     `yaml:"sub,omitempty"`
	Resp ResponsePermission `yaml:"resp,omitempty"`
}

type ResponsePermission struct {
	MaxMsgs int      `yaml:"max_msgs"`
	Expires Duration `yaml:"exp"`
}

type Limits struct {
	UserLimits jwt.UserLimits `yaml:",inline"`
	NatsLimits jwt.NatsLimits `yaml:",inline"`
}

func (c *Config) lookupAccountInfo(userAccount string) (*UserAccountInfo, error) {
	for _, acinfo := range c.Rbac.Accounts {
		if acinfo.Name == userAccount {
			return &acinfo, nil
		}
	}

	return nil, fmt.Errorf("unknown user-account: %s", userAccount)
}

// evaluateMatchCriterion checks if a single Match criterion is met by the context.
// It returns true if matched, along with a string describing the match, otherwise false and an empty string.
func evaluateMatchCriterion(match Match, context map[string]interface{}, bindingIndex int) (matched bool, description string) {
	// Handle permission-based matching
	if match.Permission != "" {
		isPermissionMatched := false
		if permissions, ok := context["permissions"].([]interface{}); ok {
			for _, p := range permissions {
				if permission, ok := p.(string); ok && permission == match.Permission {
					isPermissionMatched = true
					break
				}
			}
		} else if permission, ok := context["permissions"].(string); ok && permission == match.Permission {
			isPermissionMatched = true
		}

		if isPermissionMatched {
			log.Debug().Msgf("match-pass[permission]: %s (Binding Index: %d)", match.Permission, bindingIndex)
			return true, fmt.Sprintf("permission=%s", match.Permission)
		} else {
			log.Debug().Msgf("match-fail[permission]: %s (Binding Index: %d)", match.Permission, bindingIndex)
			return false, ""
		}
	}

	// Handle regular claim-based matching
	contextValue, exists := context[match.Claim]
	if !exists {
		log.Trace().Msgf("match-skip[%s]: claim key not found in context (Binding Index: %d)", match.Claim, bindingIndex)
		return false, "" // Claim doesn't exist, so it's not a match for this criterion
	}

	isClaimMatched := false
	switch v := contextValue.(type) {
	case string:
		if v == match.Value {
			isClaimMatched = true
			log.Debug().Msgf("match-pass[%s]: %s == %s (Binding Index: %d)", match.Claim, match.Value, v, bindingIndex)
		}
	case []interface{}:
		for _, val := range v {
			if sv, ok := val.(string); ok && sv == match.Value {
				isClaimMatched = true
				log.Debug().Msgf("match-pass[%s]: %s == %s (Binding Index: %d)", match.Claim, match.Value, val, bindingIndex)
				break
			}
		}
	default:
		log.Trace().Msgf("match-skip[%s]: unsupported type %T (Binding Index: %d)", match.Claim, v, bindingIndex)
		// Unsupported type cannot match the string value
	}

	if isClaimMatched {
		return true, fmt.Sprintf("%s=%s", match.Claim, match.Value)
	} else {
		log.Debug().Msgf("match-fail[%s]: value '%s' not found in context value '%v' (Binding Index: %d)", match.Claim, match.Value, contextValue, bindingIndex)
		return false, ""
	}
}

func (c *Config) lookupUserAccount(context map[string]interface{}) (string, *jwt.Permissions, *jwt.Limits, Duration) {
	type matchResult struct {
		matches          int
		numMatchCriteria int // Store the number of criteria in the matched binding
		account          string
		permissions      *jwt.Permissions
		limits           *jwt.Limits
		maxExpiry        Duration
		roleBindingName  string // Use a descriptive name, account might not be unique
		matchedOn        []string
	}

	var bestMatch matchResult

	strategy := c.Rbac.RoleBindingMatchingStrategy
	log.Debug().Str("strategy", string(strategy)).Msg("Using role binding matching strategy")

	for i, roleBinding := range c.Rbac.RoleBinding {
		currentMatches := 0
		currentMatchedOn := []string{}
		numMatchCriteria := len(roleBinding.Match)

		if numMatchCriteria == 0 {
			log.Trace().Msgf("Skipping role binding index %d: no match criteria", i)
			continue
		}

		// Evaluate all match criteria for this binding
		bindingFullyMatched := true // Assume full match for strict initially
		for _, match := range roleBinding.Match {
			matched, description := evaluateMatchCriterion(match, context, i)

			if matched {
				currentMatches++
				currentMatchedOn = append(currentMatchedOn, description)
			} else {
				// If even one criterion fails, it's not a full match for strict strategy
				bindingFullyMatched = false
				// For strict matching, if one fails, we can stop checking this binding
				if strategy == StrategyStrict {
					currentMatches = -1 // Mark as failed for strict comparison later
					break               // Stop checking criteria for this binding
				}
			}
		}

		// --- Strategy-based selection ---

		if strategy == StrategyStrict {
			// Check if the loop completed without breaking (currentMatches != -1)
			// and if all criteria were actually checked and matched.
			// We use bindingFullyMatched which covers the case where the loop didn't break
			// because no criterion failed, but also ensures all criteria were indeed met.
			if bindingFullyMatched && currentMatches == numMatchCriteria {
				// Strict match found! Use the first one found
				log.Debug().
					Int("matched_count", currentMatches).
					Int("required_count", numMatchCriteria).
					Int("binding_index", i).
					Str("role_binding_account", roleBinding.Account).
					Strs("matched_on", currentMatchedOn).
					Msg("selected first strictly matching role binding")

				permissions, limits := c.collateRoles(roleBinding.Roles)
				return roleBinding.Account, permissions, limits, roleBinding.TokenMaxExpiry
			}
			// If not a full match in strict mode, continue to the next binding
			continue
		}

		// best_match strategy
		if currentMatches > 0 { // Only consider bindings with at least one match
			updateBestMatch := false
			if currentMatches > bestMatch.matches {
				// More matches than current best
				updateBestMatch = true
			} else if currentMatches == bestMatch.matches {
				// Same number of matches, check the number of criteria
				if numMatchCriteria > bestMatch.numMatchCriteria {
					// More specific match (more criteria)
					updateBestMatch = true
				}
				// If numMatchCriteria is also the same, the first one encountered wins (no need to update)
			}

			if updateBestMatch {
				permissions, limits := c.collateRoles(roleBinding.Roles)
				bestMatch = matchResult{
					matches:          currentMatches,
					numMatchCriteria: numMatchCriteria, // Store the number of criteria
					account:          roleBinding.Account,
					permissions:      permissions,
					limits:           limits,
					maxExpiry:        roleBinding.TokenMaxExpiry,
					roleBindingName:  fmt.Sprintf("%s (Index: %d)", roleBinding.Account, i), // More descriptive name
					matchedOn:        currentMatchedOn,
				}
				log.Debug().Msgf("new best match found (Index: %d, Matches: %d, Criteria: %d)", i, currentMatches, numMatchCriteria)
			}
		}
	}

	// --- Final Return Logic ---

	if strategy == StrategyStrict {
		// If we finished the loop in strict mode, no binding fully matched
		log.Error().Msgf("no role-binding strictly matched idp token, context=%v", context)
		return "", nil, nil, Duration{}
	}

	// best_match: Check if any match was found
	if bestMatch.matches == 0 {
		log.Error().Msgf("no role-binding matched idp token using best_match strategy, context=%v", context)
		return "", nil, nil, Duration{}
	}

	log.Debug().
		Int("matches", bestMatch.matches).
		Int("criteria_count", bestMatch.numMatchCriteria).
		Str("role_binding", bestMatch.roleBindingName).
		Strs("matched_on", bestMatch.matchedOn).
		Msg("selected role binding using best_match strategy")

	return bestMatch.account, bestMatch.permissions, bestMatch.limits, bestMatch.maxExpiry
}

func (c *Config) collateRoles(roles []string) (*jwt.Permissions, *jwt.Limits) {
	allPermissions := jwt.Permissions{
		Resp: &jwt.ResponsePermission{
			Expires: 0,
			MaxMsgs: 0,
		},
	}

	allLimits := jwt.Limits{
		UserLimits: jwt.UserLimits{
			Src:    jwt.CIDRList{},
			Times:  nil,
			Locale: "",
		},
		NatsLimits: jwt.NatsLimits{
			Subs:    jwt.NoLimit,
			Data:    jwt.NoLimit,
			Payload: jwt.NoLimit,
		},
	}

	for _, roleName := range roles {
		role := c.lookupRole(roleName)

		log.Trace().Msgf(
			"-- assigning role [%s]: permissions=%v, limits=%v",
			roleName,
			string(internal.IgnoreError(json.Marshal(role.Permissions))),
			string(internal.IgnoreError(json.Marshal(role.Limits))),
		)

		collatePermissions(&allPermissions, &role.Permissions)
		collateLimits(&allLimits, &role.Limits)
	}

	log.Debug().Msgf(
		"-- collatedRoles: permissions=%v, limits=%v",
		string(internal.IgnoreError(json.Marshal(allPermissions))),
		string(internal.IgnoreError(json.Marshal(allLimits))),
	)

	return &allPermissions, &allLimits
}

func collateLimits(base *jwt.Limits, other *Limits) {
	base.UserLimits.Src.Add(other.UserLimits.Src...)
	base.UserLimits.Times = other.UserLimits.Times
	base.UserLimits.Locale = other.UserLimits.Locale

	base.NatsLimits.Subs = other.NatsLimits.Subs
	base.NatsLimits.Data = other.NatsLimits.Data
	base.NatsLimits.Payload = other.NatsLimits.Payload
}

func collatePermissions(base *jwt.Permissions, other *Permissions) {
	base.Pub.Allow.Add(other.Pub.Allow...)
	base.Pub.Deny.Add(other.Pub.Deny...)

	base.Sub.Allow.Add(other.Sub.Allow...)
	base.Sub.Deny.Add(other.Sub.Deny...)

	if other.Resp.Expires.Duration > 0 {
		base.Resp.Expires = other.Resp.Expires.Duration
	}
	if other.Resp.MaxMsgs > 0 {
		base.Resp.MaxMsgs = other.Resp.MaxMsgs
	}
}

func (c *Config) lookupRole(roleName string) *Role {
	for _, role := range c.Rbac.Roles {
		if role.Name == roleName {
			return &role
		}
	}

	log.Error().Msgf("unknown role: %s", roleName)
	return nil
}
