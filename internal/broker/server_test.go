package server

import (
	"testing"
	"time"
)

func Test_calculateExpiration(t *testing.T) {
	type args struct {
		cfg                       *Config
		idpProvidedExpiry         int64
		idpValidationExpiry       *DurationBounds
		roleBindingTokenMaxExpiry *Duration
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "only IDP provided expiry",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry:         time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(30 * time.Minute).Unix(),
		},
		{
			name: "IDP expiry below NATS min bound",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 30 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry:         time.Now().Add(1 * time.Minute).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(30 * time.Minute).Unix(),
		},
		{
			name: "IDP expiry above NATS max bound",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry:         time.Now().Add(2 * time.Hour).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(1 * time.Hour).Unix(),
		},
		{
			name: "IDP validation expiry min bound enforced",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry: time.Now().Add(5 * time.Minute).Unix(),
				idpValidationExpiry: &DurationBounds{
					Min: Duration{Duration: 15 * time.Minute},
					Max: Duration{Duration: 45 * time.Minute},
				},
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(15 * time.Minute).Unix(),
		},
		{
			name: "IDP validation expiry max bound enforced",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 2 * time.Hour},
						},
					},
				},
				idpProvidedExpiry: time.Now().Add(1 * time.Hour).Unix(),
				idpValidationExpiry: &DurationBounds{
					Min: Duration{Duration: 15 * time.Minute},
					Max: Duration{Duration: 45 * time.Minute},
				},
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(45 * time.Minute).Unix(),
		},
		{
			name: "role binding token max expiry overrides IDP expiry",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 2 * time.Hour},
						},
					},
				},
				idpProvidedExpiry:         time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: &Duration{Duration: 1 * time.Hour},
			},
			want: time.Now().Add(1 * time.Hour).Unix(),
		},
		{
			name: "RBAC token max expiry limits IDP expiry",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 2 * time.Hour},
						},
					},
					Rbac: Rbac{
						TokenMaxExpiry: Duration{Duration: 45 * time.Minute},
					},
				},
				idpProvidedExpiry:         time.Now().Add(1 * time.Hour).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(45 * time.Minute).Unix(),
		},
		{
			name: "all bounds interact correctly",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 5 * time.Minute},
							Max: Duration{Duration: 2 * time.Hour},
						},
					},
					Rbac: Rbac{
						TokenMaxExpiry: Duration{Duration: 1 * time.Hour},
					},
				},
				idpProvidedExpiry: time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry: &DurationBounds{
					Min: Duration{Duration: 15 * time.Minute},
					Max: Duration{Duration: 45 * time.Minute},
				},
				roleBindingTokenMaxExpiry: &Duration{Duration: 35 * time.Minute},
			},
			want: time.Now().Add(35 * time.Minute).Unix(),
		},
		{
			name: "zero IDP validation bounds are ignored",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry: time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry: &DurationBounds{
					Min: Duration{Duration: 0},
					Max: Duration{Duration: 0},
				},
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(30 * time.Minute).Unix(),
		},
		{
			name: "zero RBAC token max expiry is ignored",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
					Rbac: Rbac{
						TokenMaxExpiry: Duration{Duration: 0},
					},
				},
				idpProvidedExpiry:         time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: nil,
			},
			want: time.Now().Add(30 * time.Minute).Unix(),
		},
		{
			name: "zero role binding token max expiry is ignored",
			args: args{
				cfg: &Config{
					NATS: NATS{
						TokenExpiryBounds: DurationBounds{
							Min: Duration{Duration: 1 * time.Minute},
							Max: Duration{Duration: 1 * time.Hour},
						},
					},
				},
				idpProvidedExpiry:         time.Now().Add(30 * time.Minute).Unix(),
				idpValidationExpiry:       nil,
				roleBindingTokenMaxExpiry: &Duration{Duration: 0},
			},
			want: time.Now().Add(30 * time.Minute).Unix(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Account for small timing differences in test execution
			got := calculateExpiration(tt.args.cfg, tt.args.idpProvidedExpiry, tt.args.idpValidationExpiry, tt.args.roleBindingTokenMaxExpiry)
			// Allow for 1 second difference due to test execution time
			if diff := got - tt.want; diff < -1 || diff > 1 {
				t.Errorf("calculateExpiration() = %v, want %v (diff: %v)", got, tt.want, diff)
			}
		})
	}
}
