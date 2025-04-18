package server

import (
	"fmt"
	"strings"
	"testing"
)

func ExampleSecureLogKey() {
	// Private key (starting with P) - very sensitive, only 'P' visible
	privateKey := "PA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI"
	fmt.Println(SecureLogKey(privateKey))

	// Seed key (starting with S) - show first 2 chars (second char indicates key type)
	seedKey := "SUAIO5FWOAINMXJ2ROP4HJFQESQDIP2KSBDY7U66CZ2IMZMOWGLQBYY636"
	fmt.Println(SecureLogKey(seedKey))

	// Account key (starting with A) - public key, show in full
	accountKey := "AA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI"
	fmt.Println(SecureLogKey(accountKey))

	// Server key (starting with N) - public key, show in full
	serverKey := "NA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI"
	fmt.Println(SecureLogKey(serverKey))

	// Regular API key - first 2 chars visible
	apiKey := "api_d9bf22f8a77e4b6e9041cbef75c32410"
	fmt.Println(SecureLogKey(apiKey))

	// Output:
	// P*******************************************************
	// SU********************************************************
	// AA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI
	// NA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI
	// api_d9bf22f8a77e4b6e9041cbef75c32410
}

func TestSecureLogKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "private key",
			input:    "PA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "P" + strings.Repeat("*", 55),
		},
		{
			name:     "seed key",
			input:    "SUAIO5FWOAINMXJ2ROP4HJFQESQDIP2KSBDY7U66CZ2IMZMOWGLQBYY636",
			expected: "SU" + strings.Repeat("*", 56),
		},
		{
			name:     "account key",
			input:    "AA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "AA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "user key",
			input:    "UB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "UB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "server key",
			input:    "NA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "NA5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "cluster key",
			input:    "CB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "CB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "operator key",
			input:    "OB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "OB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "curve key",
			input:    "XB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
			expected: "XB5IBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OI",
		},
		{
			name:     "regular key",
			input:    "thisIsARegularKey12345",
			expected: "thisIsARegularKey12345",
		},
		{
			name:     "short key (1 char)",
			input:    "a",
			expected: "a",
		},
		{
			name:     "short key (2 chars)",
			input:    "ab",
			expected: "ab",
		},
		{
			name:     "short key with NATS prefix but no more chars",
			input:    "S",
			expected: "S",
		},
		{
			name:     "key with NATS prefix and one more char",
			input:    "S1",
			expected: "S1",
		},
		{
			name:     "API key example",
			input:    "sk_live_1234567890abcdefghijklmn",
			expected: "sk_live_1234567890abcdefghijklmn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureLogKey(tt.input)
			if result != tt.expected {
				t.Errorf("SecureLogKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSecureLogKeySensitiveKeys ensures we don't accidentally expose sensitive parts
// for seed keys and private keys
func TestSecureLogKeySensitiveKeys(t *testing.T) {
	// Test seed key
	seedKey := "SUAIB63KQVTAYDNMFPG424SKXP7NRUOEWGN6TDDSUUC5QMZMB127DZQQZI"
	seedResult := SecureLogKey(seedKey)

	// The result should show the first two characters 'SU'
	if len(seedResult) < 2 || seedResult[0:2] != "SU" {
		t.Errorf("First two characters should be 'SU' but got %q", seedResult[0:2])
	}

	for i := 2; i < len(seedResult); i++ {
		if seedResult[i] != '*' {
			t.Errorf("Character at position %d should be '*' but got %q", i, seedResult[i])
		}
	}

	// Test private key
	privateKey := "PAIBNECSXBKO4QCEJKO7ILXBWDQJHHH46K3D2MN6ROJQSNH7IVPQ7OIABC"
	privateResult := SecureLogKey(privateKey)

	// The result should only show the first character 'P'
	if privateResult[0] != 'P' {
		t.Errorf("First character should be 'P' but got %q", privateResult[0])
	}

	for i := 1; i < len(privateResult); i++ {
		if privateResult[i] != '*' {
			t.Errorf("Character at position %d should be '*' but got %q", i, privateResult[i])
		}
	}
}
