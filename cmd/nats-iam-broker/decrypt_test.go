package main

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryDecodeNATSJWT(t *testing.T) {
	// Create a real NATS JWT using nkeys
	kp, err := nkeys.CreateAccount()
	require.NoError(t, err)

	userKp, err := nkeys.CreateUser()
	require.NoError(t, err)
	userPub, err := userKp.PublicKey()
	require.NoError(t, err)

	claims := jwt.NewUserClaims(userPub)
	claims.Name = "test-user"
	claims.Issuer, err = kp.PublicKey()
	require.NoError(t, err)
	claims.IssuerAccount = claims.Issuer

	token, err := claims.Encode(kp)
	require.NoError(t, err)

	decoded, err := tryDecodeNATSJWT(token)
	require.NoError(t, err)
	assert.Contains(t, decoded, `"name": "test-user"`)
	assert.Contains(t, decoded, `"type": "user"`)
	assert.Contains(t, decoded, fmt.Sprintf(`"subject": "%s"`, userPub))
}

func TestTryDecodeNATSJWT_InvalidToken(t *testing.T) {
	_, err := tryDecodeNATSJWT("not-a-jwt")
	assert.Error(t, err)
}

func TestDecodeJWEHeader(t *testing.T) {
	// Build a real JWE so the header is valid
	key := make([]byte, 64) // A256CBC-HS512 needs 64 bytes
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := jose.NewEncrypter(
		jose.A256CBC_HS512,
		jose.Recipient{Algorithm: jose.DIRECT, Key: key},
		(&jose.EncrypterOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	jwe, err := enc.Encrypt([]byte(`{"hello":"world"}`))
	require.NoError(t, err)

	token, err := jwe.CompactSerialize()
	require.NoError(t, err)

	header, err := decodeJWEHeader(token)
	require.NoError(t, err)
	assert.Equal(t, "dir", header["alg"])
	assert.Equal(t, "A256CBC-HS512", header["enc"])
}

func TestDecryptJWE(t *testing.T) {
	key := make([]byte, 64)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := jose.NewEncrypter(
		jose.A256CBC_HS512,
		jose.Recipient{Algorithm: jose.DIRECT, Key: key},
		nil,
	)
	require.NoError(t, err)

	payload := []byte(`{"sub":"test-user","name":"Test"}`)
	jwe, err := enc.Encrypt(payload)
	require.NoError(t, err)

	token, err := jwe.CompactSerialize()
	require.NoError(t, err)

	plaintext, err := decryptJWE(token, key)
	require.NoError(t, err)
	assert.Equal(t, payload, plaintext)
}

func TestDecryptJWE_WrongKey(t *testing.T) {
	key := make([]byte, 64)
	wrongKey := make([]byte, 64)
	for i := range wrongKey {
		wrongKey[i] = byte(i + 1)
	}

	enc, err := jose.NewEncrypter(
		jose.A256CBC_HS512,
		jose.Recipient{Algorithm: jose.DIRECT, Key: key},
		nil,
	)
	require.NoError(t, err)

	jwe, err := enc.Encrypt([]byte("secret"))
	require.NoError(t, err)

	token, err := jwe.CompactSerialize()
	require.NoError(t, err)

	_, err = decryptJWE(token, wrongKey)
	assert.Error(t, err)
}

func TestFormatUnixTime(t *testing.T) {
	assert.Equal(t, "", formatUnixTime(0))
	assert.Equal(t, "2024-01-01T00:00:00Z", formatUnixTime(1704067200))
}

func TestDecryptJWE_WithKid(t *testing.T) {
	// Simulate the real-world scenario with a kid in the header
	key := make([]byte, 64)
	for i := range key {
		key[i] = byte(i * 3)
	}

	kid := base64.RawURLEncoding.EncodeToString([]byte("test-key-identifier"))
	enc, err := jose.NewEncrypter(
		jose.A256CBC_HS512,
		jose.Recipient{Algorithm: jose.DIRECT, Key: key},
		(&jose.EncrypterOptions{}).WithHeader(jose.HeaderKey("kid"), kid),
	)
	require.NoError(t, err)

	payload := []byte(`{"iss":"AAAA","sub":"UBBBB","nats":{"type":"user"}}`)
	jwe, err := enc.Encrypt(payload)
	require.NoError(t, err)

	token, err := jwe.CompactSerialize()
	require.NoError(t, err)

	// Verify header contains kid
	header, err := decodeJWEHeader(token)
	require.NoError(t, err)
	assert.Equal(t, kid, header["kid"])

	// Verify decryption
	plaintext, err := decryptJWE(token, key)
	require.NoError(t, err)
	assert.Equal(t, payload, plaintext)
}
