package server

import (
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func TestValidateAndSign(t *testing.T) {
	// Helper function to create fresh key pairs for each test
	createKeyPairs := func() (nkeys.KeyPair, nkeys.KeyPair, string, string) {
		accountKP, err := nkeys.CreateAccount()
		if err != nil {
			t.Fatalf("Failed to create account key pair: %v", err)
		}
		if accountKP == nil {
			t.Fatal("Created account key pair is nil")
		}

		userKP, err := nkeys.CreateUser()
		if err != nil {
			t.Fatalf("Failed to create user key pair: %v", err)
		}
		if userKP == nil {
			t.Fatal("Created user key pair is nil")
		}

		userPub, err := userKP.PublicKey()
		if err != nil {
			t.Fatalf("Failed to get user public key: %v", err)
		}
		if userPub == "" {
			t.Fatal("User public key is empty")
		}

		accountPub, err := accountKP.PublicKey()
		if err != nil {
			t.Fatalf("Failed to get account public key: %v", err)
		}
		if accountPub == "" {
			t.Fatal("Account public key is empty")
		}

		return accountKP, userKP, accountPub, userPub
	}

	type args struct {
		claims      *jwt.UserClaims
		kp          nkeys.KeyPair
		accountInfo *UserAccountInfo
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid user claims",
			args: func() args {
				accountKP, userKP, accountPub, userPub := createKeyPairs()
				defer userKP.Wipe()
				claims := jwt.NewUserClaims(userPub)
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub
				return args{
					claims:      claims,
					kp:          accountKP,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: false,
		},
		{
			name: "nil claims",
			args: func() args {
				accountKP, _, _, _ := createKeyPairs()
				return args{
					claims:      nil,
					kp:          accountKP,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
			errMsg:  "claims cannot be nil",
		},
		{
			name: "nil keypair",
			args: func() args {
				accountKP, userKP, accountPub, userPub := createKeyPairs()
				defer accountKP.Wipe()
				defer userKP.Wipe()
				claims := jwt.NewUserClaims(userPub)
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub
				return args{
					claims:      claims,
					kp:          nil,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
			errMsg:  "keypair cannot be nil",
		},
		{
			name: "empty subject",
			args: func() args {
				accountKP, _, accountPub, _ := createKeyPairs()
				claims := &jwt.UserClaims{}
				claims.Subject = ""
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub
				return args{
					claims:      claims,
					kp:          accountKP,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
		},
		{
			name: "invalid issuer account",
			args: func() args {
				_, userKP, _, userPub := createKeyPairs()
				defer userKP.Wipe()
				claims := jwt.NewUserClaims(userPub)
				claims.Name = "Test User"
				claims.IssuerAccount = "invalid_account"
				return args{
					claims: claims,
					kp: func() nkeys.KeyPair {
						accountKP, _, _, _ := createKeyPairs()
						return accountKP
					}(),
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
		},
		{
			name: "mismatched issuer and signing key",
			args: func() args {
				accountKP1, userKP, accountPub1, userPub := createKeyPairs()
				defer userKP.Wipe()

				claims := jwt.NewUserClaims(userPub)
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub1

				accountKP2, _, _, _ := createKeyPairs()
				defer accountKP1.Wipe()

				return args{
					claims:      claims,
					kp:          accountKP2,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
		},
		{
			name: "invalid user public key format",
			args: func() args {
				accountKP, _, accountPub, _ := createKeyPairs()
				claims := jwt.NewUserClaims("invalid_user_key")
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub
				return args{
					claims:      claims,
					kp:          accountKP,
					accountInfo: nil,
				}
			}(),
			want:    "",
			wantErr: true,
		},
		{
			name: "valid with account signing key",
			args: func() args {
				// Create account keypair and user keypair
				accountKP, userKP, accountPub, userPub := createKeyPairs()
				defer userKP.Wipe()
				defer accountKP.Wipe()

				// Create a separate signing keypair
				signingKP, err := nkeys.CreateAccount()
				if err != nil {
					t.Fatalf("Failed to create signing key pair: %v", err)
				}

				// Create user claims with account as issuer
				claims := jwt.NewUserClaims(userPub)
				claims.Name = "Test User"
				claims.IssuerAccount = accountPub

				// Create a UserAccountInfo with the signing key
				accountInfo := &UserAccountInfo{
					Name:      "TestAccount",
					PublicKey: accountPub,
					SigningNKey: NKey{
						KeyPair: signingKP,
					},
				}

				return args{
					claims:      claims,
					kp:          signingKP, // Use signing key instead of account key
					accountInfo: accountInfo,
				}
			}(),
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAndSign(tt.args.claims, tt.args.kp, tt.args.accountInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndSign() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Error("ValidateAndSign() returned empty JWT for valid claims")
				return
			}
			if !tt.wantErr {
				// Verify the JWT can be decoded
				_, err = jwt.DecodeUserClaims(got)
				if err != nil {
					t.Errorf("Failed to decode generated JWT: %v", err)
					return
				}
			}
			if tt.wantErr && got != tt.want {
				t.Errorf("ValidateAndSign() = %v, want %v", got, tt.want)
			}

			// Clean up the key pair after each test
			if tt.args.kp != nil {
				tt.args.kp.Wipe()
			}
		})
	}
}
