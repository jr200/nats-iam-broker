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

func runDecrypt(args []string) int {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	keyStr := fs.String("key", "", "symmetric key in base64url encoding (64 bytes for A256CBC-HS512)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s decrypt [--key <base64url-key>] <jwe-token>\n\n", os.Args[0])
		fmt.Fprintf(fs.Output(), "Decrypt a JWE-encrypted NATS JWT token.\n\n")
		fmt.Fprintf(fs.Output(), "If --key is not provided, only the JWE header is displayed.\n\n")
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

	if *keyStr == "" {
		fmt.Fprintf(os.Stdout, "\nNo --key provided; only header shown. Provide --key to decrypt payload.\n")
		return 0
	}

	key, err := base64.RawURLEncoding.DecodeString(*keyStr)
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
	// Split to extract just the first segment (header)
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
