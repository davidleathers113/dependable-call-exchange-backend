package adapters

import (
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

func TestEmailAdapter(t *testing.T) {
	adapter := NewEmailAdapter()

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		want    string
	}{
		{
			name:  "valid email string",
			input: "test@example.com",
			want:  "test@example.com",
		},
		{
			name:  "valid email bytes",
			input: []byte("user@domain.org"),
			want:  "user@domain.org",
		},
		{
			name:  "nil value",
			input: nil,
			want:  "",
		},
		{
			name:    "invalid email",
			input:   "invalid-email",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var email values.Email
			err := adapter.Scan(&email, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if email.String() != tt.want {
				t.Errorf("got %q, want %q", email.String(), tt.want)
			}

			// Test round trip
			if !email.IsEmpty() {
				value, err := adapter.Value(email)
				if err != nil {
					t.Errorf("Value() error: %v", err)
				}
				if value != tt.want {
					t.Errorf("Value() got %v, want %v", value, tt.want)
				}
			}
		})
	}
}

func TestMoneyAdapter(t *testing.T) {
	adapter := NewMoneyAdapter()

	tests := []struct {
		name     string
		input    interface{}
		wantErr  bool
		amount   float64
		currency string
	}{
		{
			name:     "JSON format",
			input:    `{"amount":"123.45","currency":"USD"}`,
			amount:   123.45,
			currency: "USD",
		},
		{
			name:     "simple decimal string",
			input:    "99.99",
			amount:   99.99,
			currency: "USD", // fallback currency
		},
		{
			name:     "nil value",
			input:    nil,
			amount:   0,
			currency: "USD", // Zero() returns USD
		},
		{
			name:    "unsupported type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var money values.Money
			err := adapter.Scan(&money, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if money.ToFloat64() != tt.amount {
				t.Errorf("got amount %f, want %f", money.ToFloat64(), tt.amount)
			}

			if money.Currency() != tt.currency {
				t.Errorf("got currency %q, want %q", money.Currency(), tt.currency)
			}

			// Test round trip (only for non-zero values)
			if !money.IsZero() {
				value, err := adapter.Value(money)
				if err != nil {
					t.Errorf("Value() error: %v", err)
				}
				if value == nil {
					t.Errorf("Value() returned nil for non-zero money")
				}
			}
		})
	}
}

func TestPhoneAdapter(t *testing.T) {
	adapter := NewPhoneAdapter()

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		want    string
	}{
		{
			name:  "E.164 format",
			input: "+12345678901",
			want:  "+12345678901",
		},
		{
			name:  "US format",
			input: "(555) 123-4567",
			want:  "+15551234567",
		},
		{
			name:  "nil value",
			input: nil,
			want:  "",
		},
		{
			name:    "invalid phone",
			input:   "not-a-phone",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var phone values.PhoneNumber
			err := adapter.Scan(&phone, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if phone.String() != tt.want {
				t.Errorf("got %q, want %q", phone.String(), tt.want)
			}

			// Test round trip
			if !phone.IsEmpty() {
				value, err := adapter.Value(phone)
				if err != nil {
					t.Errorf("Value() error: %v", err)
				}
				if value != tt.want {
					t.Errorf("Value() got %v, want %v", value, tt.want)
				}
			}
		})
	}
}

func TestQualityMetricsAdapter(t *testing.T) {
	adapter := NewQualityMetricsAdapter()

	validJSON := `{
		"quality_score": 8.5,
		"fraud_score": 1.2,
		"historical_rating": 7.8,
		"conversion_rate": 0.15,
		"average_call_time": 300,
		"trust_score": 9.0,
		"reliability_score": 8.2
	}`

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:  "valid JSON string",
			input: validJSON,
		},
		{
			name:  "valid JSON bytes",
			input: []byte(validJSON),
		},
		{
			name:  "nil value",
			input: nil,
		},
		{
			name:    "invalid JSON",
			input:   "not-json",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var metrics values.QualityMetrics
			err := adapter.Scan(&metrics, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// For nil input, should get default metrics
			if tt.input == nil {
				defaultMetrics := values.NewDefaultQualityMetrics()
				if !metrics.Equal(defaultMetrics) {
					t.Errorf("nil input should produce default metrics")
				}
			}

			// Test round trip
			value, err := adapter.Value(metrics)
			if err != nil {
				t.Errorf("Value() error: %v", err)
			}
			if value == nil {
				t.Errorf("Value() returned nil")
			}
		})
	}
}

func TestScanFromFloat64(t *testing.T) {
	adapter := NewMoneyAdapter()

	tests := []struct {
		name     string
		amount   float64
		currency string
		wantErr  bool
	}{
		{
			name:     "valid USD amount",
			amount:   123.45,
			currency: "USD",
		},
		{
			name:     "zero amount",
			amount:   0.0,
			currency: "EUR",
		},
		{
			name:     "negative amount",
			amount:   -50.25,
			currency: "GBP",
		},
		{
			name:     "invalid currency",
			amount:   100.0,
			currency: "INVALID",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var money values.Money
			err := adapter.ScanFromFloat64(&money, tt.amount, tt.currency)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if money.ToFloat64() != tt.amount {
				t.Errorf("got amount %f, want %f", money.ToFloat64(), tt.amount)
			}

			if money.Currency() != tt.currency {
				t.Errorf("got currency %q, want %q", money.Currency(), tt.currency)
			}

			// Test ValueAsFloat64
			gotAmount := adapter.ValueAsFloat64(money)
			if gotAmount != tt.amount {
				t.Errorf("ValueAsFloat64() got %f, want %f", gotAmount, tt.amount)
			}
		})
	}
}

func TestQualityMetricsIndividualFields(t *testing.T) {
	adapter := NewQualityMetricsAdapter()

	// Test ScanIndividualFields
	var metrics values.QualityMetrics
	err := adapter.ScanIndividualFields(&metrics, 8.5, 1.2, 7.8, 0.15, 300, 9.0, 8.2)
	if err != nil {
		t.Errorf("ScanIndividualFields() error: %v", err)
	}

	// Verify values
	if metrics.QualityScore != 8.5 {
		t.Errorf("got QualityScore %f, want 8.5", metrics.QualityScore)
	}
	if metrics.FraudScore != 1.2 {
		t.Errorf("got FraudScore %f, want 1.2", metrics.FraudScore)
	}

	// Test ValueIndividualFields
	qs, fs, hr, cr, act, ts, rs := adapter.ValueIndividualFields(metrics)
	if qs != 8.5 || fs != 1.2 || hr != 7.8 || cr != 0.15 || act != 300 || ts != 9.0 || rs != 8.2 {
		t.Errorf("ValueIndividualFields() returned unexpected values")
	}
}

func TestNullableAdapterMethods(t *testing.T) {
	t.Run("EmailAdapter nullable", func(t *testing.T) {
		adapter := NewEmailAdapter()

		// Test nil input
		var emailPtr *values.Email
		err := adapter.ScanNullable(&emailPtr, nil)
		if err != nil {
			t.Errorf("ScanNullable(nil) error: %v", err)
		}
		if emailPtr != nil {
			t.Errorf("expected nil pointer for nil input")
		}

		// Test valid input
		err = adapter.ScanNullable(&emailPtr, "test@example.com")
		if err != nil {
			t.Errorf("ScanNullable(valid) error: %v", err)
		}
		if emailPtr == nil {
			t.Errorf("expected non-nil pointer for valid input")
		}
		if emailPtr.String() != "test@example.com" {
			t.Errorf("got %q, want %q", emailPtr.String(), "test@example.com")
		}

		// Test ValueNullable
		value, err := adapter.ValueNullable(emailPtr)
		if err != nil {
			t.Errorf("ValueNullable() error: %v", err)
		}
		if value != "test@example.com" {
			t.Errorf("ValueNullable() got %v, want %v", value, "test@example.com")
		}

		// Test ValueNullable with nil
		value, err = adapter.ValueNullable(nil)
		if err != nil {
			t.Errorf("ValueNullable(nil) error: %v", err)
		}
		if value != nil {
			t.Errorf("ValueNullable(nil) got %v, want nil", value)
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("ScanEmail and ValueEmail", func(t *testing.T) {
		var email values.Email
		err := ScanEmail(&email, "test@example.com")
		if err != nil {
			t.Errorf("ScanEmail() error: %v", err)
		}

		value, err := ValueEmail(email)
		if err != nil {
			t.Errorf("ValueEmail() error: %v", err)
		}
		if value != "test@example.com" {
			t.Errorf("got %v, want %v", value, "test@example.com")
		}
	})

	t.Run("ScanMoney and ValueMoney", func(t *testing.T) {
		var money values.Money
		err := ScanMoney(&money, `{"amount":"123.45","currency":"USD"}`)
		if err != nil {
			t.Errorf("ScanMoney() error: %v", err)
		}

		value, err := ValueMoney(money)
		if err != nil {
			t.Errorf("ValueMoney() error: %v", err)
		}
		if value == nil {
			t.Errorf("ValueMoney() returned nil")
		}
	})
}
