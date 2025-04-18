package server

import (
	"errors"
	"fmt"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog/log"
)

// SigningKeyInfo contains information about what type of key was used to sign
type SigningKeyInfo struct {
	Type      string // "pub_key" or "signing_key"
	PublicKey string
}

type AuthService struct {
	ctx               *ServerContext
	serviceAccountKey nkeys.KeyPair
	encryptionKey     nkeys.KeyPair
	createNewClaimsFn AuthHandler
}

type AuthHandler func(req *jwt.AuthorizationRequestClaims) (*jwt.UserClaims, nkeys.KeyPair, *UserAccountInfo, error)

func NewAuthService(ctx *ServerContext, issuer nkeys.KeyPair, xkey nkeys.KeyPair, handler AuthHandler) *AuthService {
	return &AuthService{
		ctx:               ctx,
		serviceAccountKey: issuer,
		encryptionKey:     xkey,
		createNewClaimsFn: handler,
	}
}

// determineSigningKeyType checks if the signing key matches the account directly
// or if it's using the account's authorized signing key
func determineSigningKeyType(claims *jwt.UserClaims, kp nkeys.KeyPair, accountInfo *UserAccountInfo) (*SigningKeyInfo, error) {
	signingPubKey, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get signing key's public key: %v", err)
	}

	// Check if the signing key matches the issuer account directly
	if claims.IssuerAccount == signingPubKey {
		log.Trace().Msg("signing key matches account public key directly")
		return &SigningKeyInfo{
			Type:      "pub_key",
			PublicKey: signingPubKey,
		}, nil
	} else if accountInfo != nil {
		// Check if the signing key matches the account's authorized signing key
		if accountInfo.SigningNKey.KeyPair != nil {
			signingNKeyPub, err := accountInfo.SigningNKey.KeyPair.PublicKey()
			if err != nil {
				return nil, fmt.Errorf("failed to get account signing key's public key: %v", err)
			}

			if signingPubKey == signingNKeyPub {
				// The signing key matches the account's authorized signing key
				log.Trace().Msg("signing key matches account signing key")
				return &SigningKeyInfo{
					Type:      "signing_key",
					PublicKey: signingNKeyPub,
				}, nil
			} else {
				return nil, fmt.Errorf("signing key does not match account public key or signing key")
			}
		} else {
			return nil, fmt.Errorf("account signing key not available and key does not match account public key")
		}
	} else {
		return nil, fmt.Errorf("issuer account does not match signing key")
	}
}

func (a *AuthService) Handle(inRequest micro.Request) {
	log.Trace().Msgf("handling request (headers): %s", SecureLogKey(inRequest.Headers()))

	var token []byte
	var err error

	xkey := inRequest.Headers().Get("Nats-Server-Xkey")
	if len(xkey) > 0 {
		if a.encryptionKey == nil {
			_ = inRequest.Error("500", "xkey not supported", nil)
			return
		}

		// Decrypt the message.
		token, err = a.encryptionKey.Open(inRequest.Data(), xkey)
		if err != nil {
			_ = inRequest.Error("500", fmt.Sprintf("error decrypting message: %s", err.Error()), nil)
			return
		}
	} else {
		token = inRequest.Data()
	}

	rc, err := jwt.DecodeAuthorizationRequestClaims(string(token))
	if err != nil {
		log.Err(err)
		_ = inRequest.Error("500", err.Error(), nil)
	}

	userNkey := rc.UserNkey
	serverID := rc.Server.ID

	var sk nkeys.KeyPair
	var accountInfo *UserAccountInfo
	claims, sk, accountInfo, err := a.createNewClaimsFn(rc)
	if err != nil {
		a.Respond(inRequest, userNkey, serverID, "", err)
		return
	}

	signedToken, err := ValidateAndSign(claims, sk, accountInfo)
	a.Respond(inRequest, userNkey, serverID, signedToken, err)
}

func (a *AuthService) Respond(req micro.Request, userNKey, serverID, userJwt string, err error) {
	rc := jwt.NewAuthorizationResponseClaims(userNKey)
	rc.Audience = serverID
	rc.Jwt = userJwt
	if err != nil {
		rc.Error = err.Error()
	}

	log.Trace().Msgf("signing response with micro-service account")
	token, err := rc.Encode(a.serviceAccountKey)
	if err != nil {
		log.Err(err).Msg("couldn't sign response")
	}

	if a.ctx.Options.LogSensitive {
		log.Debug().Msgf("minted jwt: %s", userJwt)
	}

	data := []byte(token)

	// Check if encryption is required.
	xkey := req.Headers().Get("Nats-Server-Xkey")
	if len(xkey) > 0 {
		log.Trace().Msgf("xkey encrypting response")
		data, err = a.encryptionKey.Seal(data, xkey)
		if err != nil {
			log.Err(err).Msg("couldn't xkey-encrypt payload")
			_ = req.Respond(nil)
			return
		}
	}

	_ = req.Respond(data)
}

func ValidateAndSign(claims *jwt.UserClaims, kp nkeys.KeyPair, accountInfo *UserAccountInfo) (string, error) {
	if claims == nil {
		return "", fmt.Errorf("claims cannot be nil")
	}

	if kp == nil {
		return "", fmt.Errorf("keypair cannot be nil")
	}

	// Use the shared function to determine signing key type
	_, err := determineSigningKeyType(claims, kp, accountInfo)
	if err != nil {
		return "", err
	}

	// Validate other aspects of the claims
	vr := jwt.CreateValidationResults()
	claims.Validate(vr)
	if len(vr.Errors()) > 0 {
		return "", errors.Join(vr.Errors()...)
	}

	return claims.Encode(kp)
}
