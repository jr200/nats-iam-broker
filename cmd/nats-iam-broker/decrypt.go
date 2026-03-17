package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/nats-io/jwt/v2"
)

// jwePartCount is the number of dot-separated segments in a compact JWE token.
const jwePartCount = 5

// natsJWTPartCount is the number of dot-separated segments in a NATS JWT token.
const natsJWTPartCount = 3

func runDecrypt(args []string) int {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	keyStr := fs.String("key", "", "symmetric key in base64url encoding (for JWE decryption)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s decrypt [--key <base64url-key>] <token>\n\n", os.Args[0])
		fmt.Fprintf(fs.Output(), "Decode a NATS JWT or decrypt a JWE-encrypted token.\n\n")
		fmt.Fprintf(fs.Output(), "  NATS JWT (3 parts): decoded directly, no key needed\n")
		fmt.Fprintf(fs.Output(), "  JWE token (5 parts): requires --key for decryption\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	token := fs.Arg(0)
	if token == "" {
		fs.Usage()
		return 1
	}

	parts := strings.Split(token, ".")
	switch len(parts) {
	case natsJWTPartCount:
		return decodeNATSJWT(token)
	case jwePartCount:
		return decodeJWEToken(token, *keyStr)
	default:
		fmt.Fprintf(os.Stderr, "unrecognised token format: expected %d parts (NATS JWT) or %d parts (JWE), got %d\n",
			natsJWTPartCount, jwePartCount, len(parts))
		return 1
	}
}

func decodeNATSJWT(token string) int {
	decoded, err := tryDecodeNATSJWT(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding NATS JWT: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "=== NATS JWT ===\n%s\n", decoded)
	return 0
}

func decodeJWEToken(token, keyStr string) int {
	// Always display the JWE header
	header, err := decodeJWEHeader(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding JWE header: %v\n", err)
		return 1
	}

	headerJSON, err := json.MarshalIndent(header, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling JWE header: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "=== JWE Header ===\n%s\n", headerJSON)

	if keyStr == "" {
		fmt.Fprintf(os.Stdout, "\nNo --key provided; only header shown. Provide --key to decrypt payload.\n")
		return 0
	}

	key, err := base64.RawURLEncoding.DecodeString(keyStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding key: %v\n", err)
		return 1
	}

	plaintext, err := decryptJWE(token, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decrypting token: %v\n", err)
		return 1
	}

	// Try to decode as a NATS JWT
	if decoded, err := tryDecodeNATSJWT(string(plaintext)); err == nil {
		fmt.Fprintf(os.Stdout, "\n=== Decrypted NATS JWT ===\n%s\n", decoded)
		return 0
	}

	// Try to pretty-print as generic JSON
	var raw json.RawMessage
	if err := json.Unmarshal(plaintext, &raw); err == nil {
		var buf []byte
		buf, err = json.MarshalIndent(raw, "", "  ")
		if err == nil {
			fmt.Fprintf(os.Stdout, "\n=== Decrypted Payload (JSON) ===\n%s\n", buf)
			return 0
		}
	}

	// Fall back to raw output
	fmt.Fprintf(os.Stdout, "\n=== Decrypted Payload (raw) ===\n%s\n", plaintext)
	return 0
}

func decodeJWEHeader(token string) (map[string]interface{}, error) {
	idx := strings.IndexByte(token, '.')
	if idx < 0 {
		return nil, fmt.Errorf("invalid JWE token format")
	}
	headerPart := token[:idx]

	headerBytes, err := base64.RawURLEncoding.DecodeString(headerPart)
	if err != nil {
		return nil, fmt.Errorf("invalid base64url header: %w", err)
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid JSON header: %w", err)
	}

	return header, nil
}

func decryptJWE(token string, key []byte) ([]byte, error) {
	jwe, err := jose.ParseEncrypted(token,
		[]jose.KeyAlgorithm{jose.DIRECT},
		[]jose.ContentEncryption{jose.A256CBC_HS512},
	)
	if err != nil {
		return nil, fmt.Errorf("parsing JWE: %w", err)
	}

	plaintext, err := jwe.Decrypt(key)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

func tryDecodeNATSJWT(token string) (string, error) {
	gc, err := jwt.DecodeGeneric(token)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"type":       string(gc.ClaimType()),
		"issuer":     gc.Issuer,
		"subject":    gc.Subject,
		"issued_at":  formatUnixTime(gc.IssuedAt),
		"expires":    formatUnixTime(gc.Expires),
		"not_before": formatUnixTime(gc.NotBefore),
		"name":       gc.Name,
		"audience":   gc.Audience,
		"id":         gc.ID,
		"nats":       gc.Data,
	}

	pretty, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(pretty), nil
}

func formatUnixTime(t int64) string {
	if t == 0 {
		return ""
	}
	return time.Unix(t, 0).UTC().Format(time.RFC3339)
}
