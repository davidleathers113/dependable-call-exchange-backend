package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid simple email",
			address: "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			address: "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			address: "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dots",
			address: "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			address: "user123@example.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			address: "",
			wantErr: true,
		},
		{
			name:    "missing @ symbol",
			address: "userexample.com",
			wantErr: true,
		},
		{
			name:    "missing domain",
			address: "user@",
			wantErr: true,
		},
		{
			name:    "missing local part",
			address: "@example.com",
			wantErr: true,
		},
		{
			name:    "invalid domain",
			address: "user@invalid",
			wantErr: true,
		},
		{
			name:    "multiple @ symbols",
			address: "user@@example.com",
			wantErr: true,
		},
		{
			name:    "spaces in email",
			address: "user @example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := NewEmail(tt.address)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.address, email.String())
		})
	}
}

func TestEmail_Normalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uppercase letters",
			input:    "USER@EXAMPLE.COM",
			expected: "user@example.com",
		},
		{
			name:     "mixed case",
			input:    "User@Example.Com",
			expected: "user@example.com",
		},
		{
			name:     "leading/trailing spaces",
			input:    "  user@example.com  ",
			expected: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := NewEmail(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, email.String())
		})
	}
}

func TestEmail_Properties(t *testing.T) {
	email := MustNewEmail("user@example.com")

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("Address", func(t *testing.T) {
		assert.Equal(t, "user@example.com", email.Address())
	})

	t.Run("LocalPart", func(t *testing.T) {
		assert.Equal(t, "user", email.LocalPart())
	})

	t.Run("Domain", func(t *testing.T) {
		assert.Equal(t, "example.com", email.Domain())
	})

	t.Run("IsEmpty", func(t *testing.T) {
		empty := Email{}
		assert.True(t, empty.IsEmpty())
		assert.False(t, email.IsEmpty())
	})
}

func TestEmail_Equal(t *testing.T) {
	email1 := MustNewEmail("user@example.com")
	email2 := MustNewEmail("user@example.com")
	email3 := MustNewEmail("other@example.com")

	assert.True(t, email1.Equal(email2))
	assert.False(t, email1.Equal(email3))
}

func TestEmail_IsDomainAllowed(t *testing.T) {
	email := MustNewEmail("user@example.com")
	
	allowedDomains := []string{"example.com", "test.org"}
	
	assert.True(t, email.IsDomainAllowed(allowedDomains))
	
	disallowedDomains := []string{"other.com", "test.org"}
	assert.False(t, email.IsDomainAllowed(disallowedDomains))
}

func TestEmail_IsDisposable(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		disposable bool
	}{
		{
			name:       "regular email",
			email:      "user@example.com",
			disposable: false,
		},
		{
			name:       "disposable email",
			email:      "user@10minutemail.com",
			disposable: true,
		},
		{
			name:       "another disposable email",
			email:      "test@guerrillamail.com",
			disposable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := MustNewEmail(tt.email)
			assert.Equal(t, tt.disposable, email.IsDisposable())
		})
	}
}

func TestEmail_JSON(t *testing.T) {
	email := MustNewEmail("user@example.com")

	t.Run("Marshal", func(t *testing.T) {
		data, err := json.Marshal(email)
		require.NoError(t, err)
		
		expected := `"user@example.com"`
		assert.Equal(t, expected, string(data))
	})

	t.Run("Unmarshal valid", func(t *testing.T) {
		data := `"user@example.com"`
		
		var email Email
		err := json.Unmarshal([]byte(data), &email)
		require.NoError(t, err)
		
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("Unmarshal invalid", func(t *testing.T) {
		data := `"invalid-email"`
		
		var email Email
		err := json.Unmarshal([]byte(data), &email)
		assert.Error(t, err)
	})
}

func TestEmail_Database(t *testing.T) {
	email := MustNewEmail("user@example.com")

	t.Run("Value", func(t *testing.T) {
		value, err := email.Value()
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", value)
	})

	t.Run("Value empty", func(t *testing.T) {
		empty := Email{}
		value, err := empty.Value()
		require.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("Scan valid", func(t *testing.T) {
		var email Email
		err := email.Scan("user@example.com")
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("Scan bytes", func(t *testing.T) {
		var email Email
		err := email.Scan([]byte("user@example.com"))
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("Scan nil", func(t *testing.T) {
		var email Email
		err := email.Scan(nil)
		require.NoError(t, err)
		assert.True(t, email.IsEmpty())
	})

	t.Run("Scan invalid type", func(t *testing.T) {
		var email Email
		err := email.Scan(123)
		assert.Error(t, err)
	})

	t.Run("Scan invalid email", func(t *testing.T) {
		var email Email
		err := email.Scan("invalid-email")
		assert.Error(t, err)
	})
}

func TestMustNewEmail(t *testing.T) {
	t.Run("Valid email", func(t *testing.T) {
		email := MustNewEmail("user@example.com")
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("Invalid email panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNewEmail("invalid-email")
		})
	})
}

func TestValidateEmailDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{
			name:    "valid domain",
			domain:  "gooddomain.com",
			wantErr: false,
		},
		{
			name:    "valid subdomain",
			domain:  "mail.example.com",
			wantErr: false,
		},
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			domain:  "invalid",
			wantErr: true,
		},
		{
			name:    "blocked domain",
			domain:  "example.com",
			wantErr: true, // example.com is in blocked list
		},
		{
			name:    "localhost",
			domain:  "localhost",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmailDomain(tt.domain)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEmailValidationError(t *testing.T) {
	err := EmailValidationError{
		Address: "invalid@email",
		Reason:  "missing TLD",
	}
	
	expected := "invalid email 'invalid@email': missing TLD"
	assert.Equal(t, expected, err.Error())
}