package server

import (
	"fmt"
	"strings"
)

const (
	// KeyPrefixLength is the number of characters to show from the start of a key
	KeyPrefixLength = 2
)

// SecureLogKey returns a redacted version of a key for secure logging.
// Based on NATS key prefixes from github.com/nats-io/nkeys:
// - S: Seeds (sensitive, show first 2 chars as second char indicates key type)
// - P: Private keys (sensitive, show only prefix)
// - N, C, O, A, U, X: Public keys (not sensitive, show in full)
// For general keys, shows first 2 characters followed by asterisks
func SecureLogKey(key interface{}) string {
	// Handle nil case
	if key == nil {
		return ""
	}

	// Convert interface to string if possible
	keyStr, ok := key.(string)
	if !ok {
		// For non-string types, return string representation
		return fmt.Sprintf("%v", key)
	}

	if keyStr == "" {
		return ""
	}

	// Handle NATS keys by prefix
	if len(keyStr) > 1 {
		prefix := keyStr[:1]

		// Private keys are sensitive - show only the prefix
		if prefix == "P" {
			return fmt.Sprintf("%s%s", prefix, strings.Repeat("*", len(keyStr)-1))
		}

		// Seed keys (S) need to show first two chars as the second char indicates type
		if prefix == "S" {
			if len(keyStr) <= KeyPrefixLength {
				return keyStr
			}
			return fmt.Sprintf("%s%s", keyStr[:KeyPrefixLength], strings.Repeat("*", len(keyStr)-KeyPrefixLength))
		}

		// Public keys (N, C, O, A, U, X) are not sensitive - show in full
		if strings.ContainsAny(prefix, "NACOUX") {
			return keyStr
		}
	}

	// Default case: show first 2 chars only
	if len(keyStr) <= KeyPrefixLength {
		return keyStr
	}
	// Check if string matches key format (base32 encoded)
	isBase32 := true
	for _, c := range keyStr {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", c) {
			isBase32 = false
			break
		}
	}
	if !isBase32 {
		return keyStr
	}
	return fmt.Sprintf("%s%s", keyStr[:KeyPrefixLength], strings.Repeat("*", len(keyStr)-KeyPrefixLength))
}
