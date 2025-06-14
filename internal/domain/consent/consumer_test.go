package consent

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumer(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		email       *string
		firstName   string
		lastName    string
		wantErr     bool
		errCode     string
	}{
		{
			name:        "valid consumer with phone",
			phoneNumber: "+14155551234",
			email:       nil,
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     false,
		},
		{
			name:        "valid consumer with email",
			phoneNumber: "",
			email:       stringPtr("john.doe@example.com"),
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     false,
		},
		{
			name:        "valid consumer with both phone and email",
			phoneNumber: "+14155551234",
			email:       stringPtr("john.doe@example.com"),
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     false,
		},
		{
			name:        "missing contact info",
			phoneNumber: "",
			email:       nil,
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     true,
			errCode:     "CONTACT_REQUIRED",
		},
		{
			name:        "invalid phone number",
			phoneNumber: "invalid",
			email:       nil,
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     true,
			errCode:     "INVALID_PHONE",
		},
		{
			name:        "invalid email",
			phoneNumber: "",
			email:       stringPtr("invalid-email"),
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     true,
			errCode:     "INVALID_EMAIL",
		},
		{
			name:        "email too short",
			phoneNumber: "",
			email:       stringPtr("a@"),
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     true,
			errCode:     "INVALID_EMAIL",
		},
		{
			name:        "email with multiple @ symbols",
			phoneNumber: "",
			email:       stringPtr("test@@example.com"),
			firstName:   "John",
			lastName:    "Doe",
			wantErr:     true,
			errCode:     "MULTIPLE_AT_SYMBOLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.phoneNumber, tt.email, tt.firstName, tt.lastName)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, consumer)
				assert.NotEqual(t, uuid.Nil, consumer.ID)
				assert.Equal(t, tt.firstName, consumer.FirstName)
				assert.Equal(t, tt.lastName, consumer.LastName)
				
				if tt.phoneNumber != "" {
					assert.NotNil(t, consumer.PhoneNumber)
					assert.Equal(t, tt.phoneNumber, consumer.PhoneNumber.String())
				}
				
				if tt.email != nil {
					assert.Equal(t, *tt.email, *consumer.Email)
				}
			}
		})
	}
}

func TestGetPrimaryContact(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		email       *string
		expected    string
	}{
		{
			name:        "phone as primary",
			phoneNumber: "+14155551234",
			email:       nil,
			expected:    "+14155551234",
		},
		{
			name:        "email as primary when no phone",
			phoneNumber: "",
			email:       stringPtr("john@example.com"),
			expected:    "john@example.com",
		},
		{
			name:        "phone preferred when both exist",
			phoneNumber: "+14155551234",
			email:       stringPtr("john@example.com"),
			expected:    "+14155551234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.phoneNumber, tt.email, "John", "Doe")
			require.NoError(t, err)
			
			primary := consumer.GetPrimaryContact()
			assert.Equal(t, tt.expected, primary)
		})
	}
}

func TestUpdateContact(t *testing.T) {
	tests := []struct {
		name            string
		initialPhone    string
		initialEmail    *string
		newPhone        string
		newEmail        *string
		wantErr         bool
		errCode         string
	}{
		{
			name:         "update phone number",
			initialPhone: "+14155551234",
			initialEmail: nil,
			newPhone:     "+14155555678",
			newEmail:     nil,
			wantErr:      false,
		},
		{
			name:         "update email",
			initialPhone: "+14155551234",
			initialEmail: stringPtr("old@example.com"),
			newPhone:     "+14155551234",
			newEmail:     stringPtr("new@example.com"),
			wantErr:      false,
		},
		{
			name:         "add email to phone-only consumer",
			initialPhone: "+14155551234",
			initialEmail: nil,
			newPhone:     "+14155551234",
			newEmail:     stringPtr("new@example.com"),
			wantErr:      false,
		},
		{
			name:         "invalid new phone",
			initialPhone: "+14155551234",
			initialEmail: nil,
			newPhone:     "invalid",
			newEmail:     nil,
			wantErr:      true,
			errCode:      "INVALID_PHONE",
		},
		{
			name:         "invalid new email",
			initialPhone: "+14155551234",
			initialEmail: nil,
			newPhone:     "",
			newEmail:     stringPtr("invalid-email"),
			wantErr:      true,
			errCode:      "INVALID_EMAIL",
		},
		{
			name:         "remove all contact info",
			initialPhone: "+14155551234",
			initialEmail: stringPtr("test@example.com"),
			newPhone:     "",
			newEmail:     nil,
			wantErr:      true,
			errCode:      "CONTACT_REQUIRED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.initialPhone, tt.initialEmail, "John", "Doe")
			require.NoError(t, err)
			
			originalUpdatedAt := consumer.UpdatedAt
			
			err = consumer.UpdateContact(tt.newPhone, tt.newEmail)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
				// UpdatedAt should not change on error
				assert.Equal(t, originalUpdatedAt, consumer.UpdatedAt)
			} else {
				require.NoError(t, err)
				
				if tt.newPhone != "" {
					assert.NotNil(t, consumer.PhoneNumber)
					assert.Equal(t, tt.newPhone, consumer.PhoneNumber.String())
				}
				
				if tt.newEmail != nil {
					assert.Equal(t, *tt.newEmail, *consumer.Email)
				}
				
				// UpdatedAt should be updated
				assert.True(t, consumer.UpdatedAt.After(originalUpdatedAt))
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid email",
			email:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dots",
			email:   "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			email:   "user123@example.com",
			wantErr: false,
		},
		{
			name:    "email too short",
			email:   "a@b",
			wantErr: true,
			errCode: "INVALID_EMAIL_LENGTH",
		},
		{
			name:    "email too long",
			email:   string(make([]byte, 256)),
			wantErr: true,
			errCode: "INVALID_EMAIL_LENGTH",
		},
		{
			name:    "missing @ symbol",
			email:   "invalidemail.com",
			wantErr: true,
			errCode: "INVALID_EMAIL_FORMAT",
		},
		{
			name:    "@ at beginning",
			email:   "@example.com",
			wantErr: true,
			errCode: "INVALID_EMAIL_FORMAT",
		},
		{
			name:    "@ at end",
			email:   "test@",
			wantErr: true,
			errCode: "INVALID_EMAIL_FORMAT",
		},
		{
			name:    "multiple @ symbols",
			email:   "test@@example.com",
			wantErr: true,
			errCode: "MULTIPLE_AT_SYMBOLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}