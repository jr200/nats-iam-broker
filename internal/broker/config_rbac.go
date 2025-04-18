package server

import (
	"encoding/json"
	"fmt"

	internal "github.com/jr200/nats-iam-broker/internal"

	"github.com/nats-io/jwt/v2"
	"github.com/rs/zerolog/log"
)

// Struct definitions
type Rbac struct {
	Accounts       []UserAccountInfo `yaml:"user_accounts"`
	RoleBinding    []RoleBinding     `yaml:"role_binding"`
	Roles          []Role            `yaml:"roles"`
	TokenMaxExpiry Duration          `yaml:"token_max_expiration"`
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

func (c *Config) lookupUserAccount(context map[string]interface{}) (string, *jwt.Permissions, *jwt.Limits, Duration) {
	type matchResult struct {
		matches     int
		account     string
		permissions *jwt.Permissions
		limits      *jwt.Limits
		maxExpiry   Duration
		roleBinding string
		matchedOn   []string
	}

	var bestMatch matchResult

	for _, roleBinding := range c.Rbac.RoleBinding {
		currentMatches := 0
		currentMatchedOn := []string{}

		for _, match := range roleBinding.Match {
			// Handle permission-based matching
			if match.Permission != "" {
				if permissions, ok := context["permissions"].([]interface{}); ok {
					for _, p := range permissions {
						if permission, ok := p.(string); ok && permission == match.Permission {
							currentMatches++
							currentMatchedOn = append(currentMatchedOn,
								fmt.Sprintf("permission=%s", match.Permission))
							log.Debug().Msgf("match-pass[permission]: %s", match.Permission)
							break
						}
					}
				} else if permission, ok := context["permissions"].(string); ok && permission == match.Permission {
					currentMatches++
					currentMatchedOn = append(currentMatchedOn,
						fmt.Sprintf("permission=%s", match.Permission))
					log.Debug().Msgf("match-pass[permission]: %s", match.Permission)
				}
				continue
			}

			// Handle regular claim-based matching
			contextValue, exists := context[match.Claim]
			if !exists {
				log.Trace().Msgf("match-skip[%s]: claim key not found in context", match.Claim)
				continue
			}

			switch v := contextValue.(type) {
			case string:
				if v == match.Value {
					currentMatches++
					currentMatchedOn = append(currentMatchedOn,
						fmt.Sprintf("%s=%s", match.Claim, match.Value))
					log.Debug().Msgf("match-pass[%s]: %s == %s", match.Claim, match.Value, v)
				}
			case []interface{}:
				for _, val := range v {
					if val == match.Value {
						currentMatches++
						currentMatchedOn = append(currentMatchedOn,
							fmt.Sprintf("%s=%s", match.Claim, match.Value))
						log.Debug().Msgf("match-pass[%s]: %s == %s", match.Claim, match.Value, val)
						break
					}
				}
			default:
				log.Trace().Msgf("match-skip[%s]: unsupported type %T", match.Claim, v)
			}
		}

		if currentMatches > bestMatch.matches {
			permissions, limits := c.collateRoles(roleBinding.Roles)
			bestMatch = matchResult{
				matches:     currentMatches,
				account:     roleBinding.Account,
				permissions: permissions,
				limits:      limits,
				maxExpiry:   roleBinding.TokenMaxExpiry,
				roleBinding: roleBinding.Account,
				matchedOn:   currentMatchedOn,
			}
		}
	}

	if bestMatch.matches == 0 {
		log.Error().Msgf("no role-binding matched idp token, context=%v", context)
		return "", nil, nil, Duration{}
	}

	log.Debug().
		Int("matches", bestMatch.matches).
		Str("role_binding", bestMatch.roleBinding).
		Strs("matched_on", bestMatch.matchedOn).
		Msg("selected role binding with the most matches")

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
