package server

import (
	"errors"
	"fmt"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog/log"
)

type AuthService struct {
	serviceAccountKey nkeys.KeyPair
	encryptionKey     nkeys.KeyPair
	createNewClaimsFn AuthHandler
}

type AuthHandler func(req *jwt.AuthorizationRequestClaims) (*jwt.UserClaims, nkeys.KeyPair, error)

func NewAuthService(issuer nkeys.KeyPair, xkey nkeys.KeyPair, handler AuthHandler) *AuthService {
	return &AuthService{
		serviceAccountKey: issuer,
		encryptionKey:     xkey,
		createNewClaimsFn: handler,
	}
}

func (a *AuthService) Handle(inRequest micro.Request) {
	log.Trace().Msgf("handling request (headers): %s", inRequest.Headers())

	var token []byte
	var err error

	xkey := inRequest.Headers().Get("Nats-Server-Xkey")
	if len(xkey) > 0 {
		if a.encryptionKey == nil {
			inRequest.Error("500", "xkey not supported", nil)
			return
		}

		// Decrypt the message.
		token, err = a.encryptionKey.Open(inRequest.Data(), xkey)
		if err != nil {
			inRequest.Error("500", fmt.Sprintf("error decrypting message: %s", err.Error()), nil)
			return
		}
	} else {
		token = inRequest.Data()
	}

	rc, err := jwt.DecodeAuthorizationRequestClaims(string(token))
	if err != nil {
		log.Err(err)
		inRequest.Error("500", err.Error(), nil)
	}

	userNkey := rc.UserNkey
	serverId := rc.Server.ID

	var sk nkeys.KeyPair
	claims, sk, err := a.createNewClaimsFn(rc)
	if err != nil {
		a.Respond(inRequest, userNkey, serverId, "", err)
		return
	}

	signedToken, err := ValidateAndSign(claims, sk)
	a.Respond(inRequest, userNkey, serverId, signedToken, err)
}

func (a *AuthService) Respond(req micro.Request, userNKey, serverId, userJwt string, err error) {
	rc := jwt.NewAuthorizationResponseClaims(userNKey)
	rc.Audience = serverId
	rc.Jwt = userJwt
	if err != nil {
		rc.Error = err.Error()
	}

	log.Trace().Msgf("signing response with micro-service account")
	token, err := rc.Encode(a.serviceAccountKey)
	if err != nil {
		log.Err(err).Msg("couldn't sign response")
	}

	// log.Error().Msgf("minted: %s", userJwt)

	data := []byte(token)

	// Check if encryption is required.
	xkey := req.Headers().Get("Nats-Server-Xkey")
	if len(xkey) > 0 {
		log.Trace().Msgf("xkey encrypting response")
		data, err = a.encryptionKey.Seal(data, xkey)
		if err != nil {
			log.Err(err).Msg("couldn't xkey-encrypt payload")
			req.Respond(nil)
			return
		}
	}

	req.Respond(data)
}

func ValidateAndSign(claims *jwt.UserClaims, kp nkeys.KeyPair) (string, error) {
	// Validate the claims.
	vr := jwt.CreateValidationResults()
	claims.Validate(vr)
	if len(vr.Errors()) > 0 {
		return "", errors.Join(vr.Errors()...)
	}

	return claims.Encode(kp)
}
