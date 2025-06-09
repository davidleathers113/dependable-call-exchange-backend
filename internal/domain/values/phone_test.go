package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid E.164 US number",
			number:   "+15551234567",
			expected: "+15551234567",
			wantErr:  false,
		},
		{
			name:     "US number with parentheses",
			number:   "(555) 123-4567",
			expected: "+15551234567",
			wantErr:  false,
		},
		{
			name:     "US number with dashes",
			number:   "555-123-4567",
			expected: "+15551234567",
			wantErr:  false,
		},
		{
			name:     "US number with spaces",
			number:   "555 123 4567",
			expected: "+15551234567",
			wantErr:  false,
		},
		{
			name:     "US number with country code",
			number:   "1-555-123-4567",
			expected: "+15551234567",
			wantErr:  false,
		},
		{
			name:     "international UK number",
			number:   "+442071234567",
			expected: "+442071234567",
			wantErr:  false,
		},
		{
			name:     "empty number",
			number:   "",
			wantErr:  true,
		},
		{
			name:     "too short",
			number:   "123",
			wantErr:  true,
		},
		{
			name:     "invalid characters",
			number:   "abc-def-ghij",
			wantErr:  true,
		},
		{
			name:     "too long",
			number:   "+1234567890123456789",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone, err := NewPhoneNumber(tt.number)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.expected, phone.String())
		})
	}
}

func TestNewPhoneNumberE164(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		wantErr bool
	}{
		{
			name:    "valid E.164",
			number:  "+15551234567",
			wantErr: false,
		},
		{
			name:    "missing plus",
			number:  "15551234567",
			wantErr: true,
		},
		{
			name:    "too long",
			number:  "+1234567890123456789",
			wantErr: true,
		},
		{
			name:    "starts with zero",
			number:  "+05551234567",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPhoneNumberE164(tt.number)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPhoneNumber_Properties(t *testing.T) {
	phone := MustNewPhoneNumber("+15551234567")

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("E164", func(t *testing.T) {
		assert.Equal(t, "+15551234567", phone.E164())
	})

	t.Run("IsEmpty", func(t *testing.T) {
		empty := PhoneNumber{}
		assert.True(t, empty.IsEmpty())
		assert.False(t, phone.IsEmpty())
	})

	t.Run("Equal", func(t *testing.T) {
		phone2 := MustNewPhoneNumber("+15551234567")
		phone3 := MustNewPhoneNumber("+15559876543")
		
		assert.True(t, phone.Equal(phone2))
		assert.False(t, phone.Equal(phone3))
	})
}

func TestPhoneNumber_USProperties(t *testing.T) {
	phone := MustNewPhoneNumber("+15551234567")

	t.Run("CountryCode", func(t *testing.T) {
		assert.Equal(t, "+1", phone.CountryCode())
	})

	t.Run("NationalNumber", func(t *testing.T) {
		assert.Equal(t, "5551234567", phone.NationalNumber())
	})

	t.Run("IsUS", func(t *testing.T) {
		assert.True(t, phone.IsUS())
		
		ukPhone := MustNewPhoneNumber("+442071234567")
		assert.False(t, ukPhone.IsUS())
	})

	t.Run("AreaCode", func(t *testing.T) {
		assert.Equal(t, "555", phone.AreaCode())
	})

	t.Run("Exchange", func(t *testing.T) {
		assert.Equal(t, "123", phone.Exchange())
	})

	t.Run("Subscriber", func(t *testing.T) {
		assert.Equal(t, "4567", phone.Subscriber())
	})
}

func TestPhoneNumber_Formatting(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		formatUS string
		formatIntl string
	}{
		{
			name:       "US number",
			number:     "+15551234567",
			formatUS:   "(555) 123-4567",
			formatIntl: "+1 555 123 4567",
		},
		{
			name:       "UK number",
			number:     "+442071234567",
			formatUS:   "+442071234567", // Should return original for non-US
			formatIntl: "+44 207 123 4567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone := MustNewPhoneNumber(tt.number)
			
			assert.Equal(t, tt.formatUS, phone.FormatUS())
			assert.Equal(t, tt.formatIntl, phone.FormatInternational())
		})
	}
}

func TestPhoneNumber_CountryCodes(t *testing.T) {
	tests := []struct {
		name        string
		number      string
		countryCode string
	}{
		{
			name:        "US/Canada",
			number:      "+15551234567",
			countryCode: "+1",
		},
		{
			name:        "UK",
			number:      "+442071234567",
			countryCode: "+44",
		},
		{
			name:        "France",
			number:      "+33123456789",
			countryCode: "+33",
		},
		{
			name:        "Germany",
			number:      "+49123456789",
			countryCode: "+49",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone := MustNewPhoneNumber(tt.number)
			assert.Equal(t, tt.countryCode, phone.CountryCode())
		})
	}
}

func TestPhoneNumber_JSON(t *testing.T) {
	phone := MustNewPhoneNumber("+15551234567")

	t.Run("Marshal", func(t *testing.T) {
		data, err := json.Marshal(phone)
		require.NoError(t, err)
		
		expected := `"+15551234567"`
		assert.Equal(t, expected, string(data))
	})

	t.Run("Unmarshal valid", func(t *testing.T) {
		data := `"+15551234567"`
		
		var phone PhoneNumber
		err := json.Unmarshal([]byte(data), &phone)
		require.NoError(t, err)
		
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Unmarshal US format", func(t *testing.T) {
		data := `"(555) 123-4567"`
		
		var phone PhoneNumber
		err := json.Unmarshal([]byte(data), &phone)
		require.NoError(t, err)
		
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Unmarshal invalid", func(t *testing.T) {
		data := `"invalid-phone"`
		
		var phone PhoneNumber
		err := json.Unmarshal([]byte(data), &phone)
		assert.Error(t, err)
	})
}

func TestPhoneNumber_Database(t *testing.T) {
	phone := MustNewPhoneNumber("+15551234567")

	t.Run("Value", func(t *testing.T) {
		value, err := phone.Value()
		require.NoError(t, err)
		assert.Equal(t, "+15551234567", value)
	})

	t.Run("Value empty", func(t *testing.T) {
		empty := PhoneNumber{}
		value, err := empty.Value()
		require.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("Scan valid", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan("+15551234567")
		require.NoError(t, err)
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Scan US format", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan("(555) 123-4567")
		require.NoError(t, err)
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Scan bytes", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan([]byte("+15551234567"))
		require.NoError(t, err)
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Scan nil", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan(nil)
		require.NoError(t, err)
		assert.True(t, phone.IsEmpty())
	})

	t.Run("Scan invalid type", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan(123)
		assert.Error(t, err)
	})

	t.Run("Scan invalid phone", func(t *testing.T) {
		var phone PhoneNumber
		err := phone.Scan("invalid-phone")
		assert.Error(t, err)
	})
}

func TestMustNewPhoneNumber(t *testing.T) {
	t.Run("Valid phone", func(t *testing.T) {
		phone := MustNewPhoneNumber("+15551234567")
		assert.Equal(t, "+15551234567", phone.String())
	})

	t.Run("Invalid phone panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNewPhoneNumber("invalid-phone")
		})
	})
}

func TestPhoneValidationError(t *testing.T) {
	err := PhoneValidationError{
		Number: "invalid-phone",
		Reason: "invalid format",
	}
	
	expected := "invalid phone number 'invalid-phone': invalid format"
	assert.Equal(t, expected, err.Error())
}

func TestPhoneNumber_HelperFunctions(t *testing.T) {
	t.Run("cleanPhoneNumber", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"(555) 123-4567", "5551234567"},
			{"+1-555-123-4567", "+15551234567"},
			{"555.123.4567", "5551234567"},
			{"555 123 4567", "5551234567"},
			{"+44 20 7123 4567", "+442071234567"},
		}

		for _, tt := range tests {
			result := cleanPhoneNumber(tt.input)
			assert.Equal(t, tt.expected, result, "cleanPhoneNumber(%s)", tt.input)
		}
	})

	t.Run("parseUSPhoneNumber", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
			valid    bool
		}{
			{"(555) 123-4567", "+15551234567", true},
			{"555-123-4567", "+15551234567", true},
			{"555.123.4567", "+15551234567", true},
			{"1-555-123-4567", "+15551234567", true},
			{"+1-555-123-4567", "+15551234567", true},
			{"invalid", "", false},
			{"123", "", false},
		}

		for _, tt := range tests {
			result, valid := parseUSPhoneNumber(tt.input)
			assert.Equal(t, tt.valid, valid, "parseUSPhoneNumber(%s) validity", tt.input)
			if valid {
				assert.Equal(t, tt.expected, result, "parseUSPhoneNumber(%s) result", tt.input)
			}
		}
	})
}