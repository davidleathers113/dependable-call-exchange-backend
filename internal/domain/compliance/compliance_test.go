package compliance_test

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestNewComplianceRule(t *testing.T) {
	tests := []struct {
		name      string
		ruleName  string
		ruleType  compliance.RuleType
		createdBy uuid.UUID
		validate  func(t *testing.T, r *compliance.ComplianceRule)
	}{
		{
			name:      "creates TCPA rule with defaults",
			ruleName:  "TCPA Compliance",
			ruleType:  compliance.RuleTypeTCPA,
			createdBy: uuid.New(),
			validate: func(t *testing.T, r *compliance.ComplianceRule) {
				assert.NotEqual(t, uuid.Nil, r.ID)
				assert.Equal(t, "TCPA Compliance", r.Name)
				assert.Equal(t, compliance.RuleTypeTCPA, r.Type)
				assert.Equal(t, compliance.RuleStatusDraft, r.Status)
				assert.Equal(t, 1, r.Priority)
				assert.NotZero(t, r.CreatedAt)
				assert.NotZero(t, r.UpdatedAt)
				assert.NotZero(t, r.EffectiveAt)
				assert.Nil(t, r.ExpiresAt)
			},
		},
		{
			name:      "creates GDPR rule",
			ruleName:  "GDPR Data Protection",
			ruleType:  compliance.RuleTypeGDPR,
			createdBy: uuid.New(),
			validate: func(t *testing.T, r *compliance.ComplianceRule) {
				assert.Equal(t, compliance.RuleTypeGDPR, r.Type)
				assert.Equal(t, "GDPR Data Protection", r.Name)
			},
		},
		{
			name:      "creates DNC rule",
			ruleName:  "Do Not Call Registry",
			ruleType:  compliance.RuleTypeDNC,
			createdBy: uuid.New(),
			validate: func(t *testing.T, r *compliance.ComplianceRule) {
				assert.Equal(t, compliance.RuleTypeDNC, r.Type)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := compliance.NewComplianceRule(tt.ruleName, tt.ruleType, tt.createdBy)
			require.NotNil(t, r)
			tt.validate(t, r)
		})
	}
}

func TestNewConsentRecord(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		consentType compliance.ConsentType
		source      string
		ipAddress   string
		userAgent   string
		validate    func(t *testing.T, c *compliance.ConsentRecord)
	}{
		{
			name:        "creates TCPA consent record",
			phoneNumber: "+15551234567",
			consentType: compliance.ConsentTypeExpress,
			source:      "web_form",
			ipAddress:   "192.168.1.100",
			userAgent:   "Mozilla/5.0",
			validate: func(t *testing.T, c *compliance.ConsentRecord) {
				assert.NotEqual(t, uuid.Nil, c.ID)
				assert.Equal(t, "+15551234567", c.PhoneNumber)
				assert.Equal(t, compliance.ConsentTypeExpress, c.ConsentType)
				assert.Equal(t, compliance.ConsentStatusActive, c.Status)
				assert.Equal(t, "web_form", c.Source)
				assert.Equal(t, "192.168.1.100", c.IPAddress)
				assert.NotZero(t, c.OptInTimestamp)
				assert.Nil(t, c.OptOutTimestamp)
				assert.Nil(t, c.ExpiresAt)
			},
		},
		{
			name:        "creates GDPR consent record",
			phoneNumber: "+442012345678",
			consentType: compliance.ConsentTypeImplied,
			source:      "mobile_app",
			ipAddress:   "10.0.0.1",
			userAgent:   "MobileApp/1.0",
			validate: func(t *testing.T, c *compliance.ConsentRecord) {
				assert.Equal(t, compliance.ConsentTypeImplied, c.ConsentType)
				assert.Equal(t, "mobile_app", c.Source)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := compliance.NewConsentRecord(tt.phoneNumber, tt.consentType, tt.source, tt.ipAddress, tt.userAgent)
			require.NotNil(t, c)
			tt.validate(t, c)
		})
	}
}

func TestConsentRecord_Revoke(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *compliance.ConsentRecord
		validate func(t *testing.T, c *compliance.ConsentRecord, oldUpdatedAt time.Time)
	}{
		{
			name: "revokes active consent",
			setup: func() *compliance.ConsentRecord {
				return fixtures.NewConsentRecordBuilder(t).
					WithStatus(compliance.ConsentStatusActive).
					Build()
			},
			validate: func(t *testing.T, c *compliance.ConsentRecord, oldUpdatedAt time.Time) {
				assert.Equal(t, compliance.ConsentStatusRevoked, c.Status)
				assert.NotNil(t, c.OptOutTimestamp)
				assert.True(t, c.UpdatedAt.After(oldUpdatedAt))
				assert.True(t, c.OptOutTimestamp.After(c.OptInTimestamp))
			},
		},
		{
			name: "revokes expired consent",
			setup: func() *compliance.ConsentRecord {
				return fixtures.NewConsentRecordBuilder(t).
					WithStatus(compliance.ConsentStatusExpired).
					Build()
			},
			validate: func(t *testing.T, c *compliance.ConsentRecord, oldUpdatedAt time.Time) {
				assert.Equal(t, compliance.ConsentStatusRevoked, c.Status)
				assert.NotNil(t, c.OptOutTimestamp)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup()
			oldUpdatedAt := c.UpdatedAt
			
			time.Sleep(10 * time.Millisecond)
			c.Revoke()
			
			tt.validate(t, c, oldUpdatedAt)
		})
	}
}

func TestRuleType_String(t *testing.T) {
	tests := []struct {
		ruleType compliance.RuleType
		expected string
	}{
		{compliance.RuleTypeTCPA, "tcpa"},
		{compliance.RuleTypeGDPR, "gdpr"},
		{compliance.RuleTypeCCPA, "ccpa"},
		{compliance.RuleTypeDNC, "dnc"},
		{compliance.RuleTypeCustom, "custom"},
		{compliance.RuleType(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ruleType.String())
		})
	}
}

func TestComplianceCheck_AddViolation(t *testing.T) {
	cc := &compliance.ComplianceCheck{
		CallID:      uuid.New(),
		PhoneNumber: "+15551234567",
		Approved:    true,
		Violations:  []compliance.ComplianceViolation{},
	}
	
	violation := compliance.ComplianceViolation{
		ID:            uuid.New(),
		CallID:        cc.CallID,
		AccountID:     uuid.New(),
		RuleID:        uuid.New(),
		ViolationType: compliance.ViolationTCPA,
		Severity:      compliance.SeverityHigh,
		Description:   "Test violation",
		CreatedAt:     time.Now(),
	}
	
	cc.AddViolation(violation)
	
	assert.Equal(t, 1, len(cc.Violations))
	assert.False(t, cc.Approved)
	assert.Equal(t, violation.ID, cc.Violations[0].ID)
}

func TestComplianceCheck_IsCallAllowed(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *compliance.ComplianceCheck
		expected bool
	}{
		{
			name: "allowed when approved with no violations",
			setup: func() *compliance.ComplianceCheck {
				return &compliance.ComplianceCheck{
					Approved:   true,
					Violations: []compliance.ComplianceViolation{},
				}
			},
			expected: true,
		},
		{
			name: "not allowed when not approved",
			setup: func() *compliance.ComplianceCheck {
				return &compliance.ComplianceCheck{
					Approved:   false,
					Violations: []compliance.ComplianceViolation{},
				}
			},
			expected: false,
		},
		{
			name: "not allowed when has violations",
			setup: func() *compliance.ComplianceCheck {
				cc := &compliance.ComplianceCheck{
					Approved:   true,
					Violations: []compliance.ComplianceViolation{},
				}
				cc.AddViolation(compliance.ComplianceViolation{
					ID:          uuid.New(),
					Description: "Test violation",
				})
				return cc
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := tt.setup()
			assert.Equal(t, tt.expected, cc.IsCallAllowed())
		})
	}
}

func TestCompliance_TimeWindows(t *testing.T) {
	t.Run("business hours window", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithTimeWindows([]compliance.TimeWindow{
				{
					StartHour: 9,
					EndHour:   17,
					Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
					Timezone:  "America/New_York",
				},
			}).
			Build()
		
		assert.Len(t, rule.TimeWindows, 1)
		assert.Equal(t, 9, rule.TimeWindows[0].StartHour)
		assert.Equal(t, 17, rule.TimeWindows[0].EndHour)
		assert.Len(t, rule.TimeWindows[0].Days, 5)
		assert.NotContains(t, rule.TimeWindows[0].Days, "Sat")
		assert.NotContains(t, rule.TimeWindows[0].Days, "Sun")
	})
	
	t.Run("multiple time windows", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithTimeWindows([]compliance.TimeWindow{
				{
					StartHour: 9,
					EndHour:   12,
					Days:      []string{"Mon", "Wed", "Fri"},
					Timezone:  "America/Los_Angeles",
				},
				{
					StartHour: 14,
					EndHour:   18,
					Days:      []string{"Tue", "Thu"},
					Timezone:  "America/Los_Angeles",
				},
			}).
			Build()
		
		assert.Len(t, rule.TimeWindows, 2)
	})
}

func TestCompliance_GeographicScope(t *testing.T) {
	t.Run("country level scope", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithGeography(compliance.GeographicScope{
				Countries: []string{"US", "CA", "MX"},
			}).
			Build()
		
		assert.Len(t, rule.Geography.Countries, 3)
		assert.Contains(t, rule.Geography.Countries, "US")
		assert.Empty(t, rule.Geography.States)
	})
	
	t.Run("state level scope", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithGeography(compliance.GeographicScope{
				Countries: []string{"US"},
				States:    []string{"CA", "TX", "FL", "NY"},
			}).
			Build()
		
		assert.Len(t, rule.Geography.States, 4)
	})
	
	t.Run("city and zip code scope", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithGeography(compliance.GeographicScope{
				Cities:   []string{"Los Angeles", "San Francisco", "San Diego"},
				ZipCodes: []string{"90001", "90002", "90003"},
			}).
			Build()
		
		assert.Len(t, rule.Geography.Cities, 3)
		assert.Len(t, rule.Geography.ZipCodes, 3)
	})
}

func TestCompliance_Conditions(t *testing.T) {
	t.Run("time of day condition", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithConditions([]compliance.Condition{
				{
					Field:    "time_of_day",
					Operator: "between",
					Value:    []int{8, 21},
				},
			}).
			Build()
		
		assert.Len(t, rule.Conditions, 1)
		assert.Equal(t, "time_of_day", rule.Conditions[0].Field)
		assert.Equal(t, "between", rule.Conditions[0].Operator)
	})
	
	t.Run("multiple conditions", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithConditions([]compliance.Condition{
				{
					Field:    "consent_status",
					Operator: "equals",
					Value:    "active",
				},
				{
					Field:    "call_frequency",
					Operator: "less_than",
					Value:    3,
				},
				{
					Field:    "account_type",
					Operator: "in",
					Value:    []string{"residential", "mobile"},
				},
			}).
			Build()
		
		assert.Len(t, rule.Conditions, 3)
	})
}

func TestCompliance_Actions(t *testing.T) {
	t.Run("block action", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithActions([]compliance.Action{
				{
					Type:   compliance.ActionBlock,
					Params: nil,
				},
			}).
			Build()
		
		assert.Len(t, rule.Actions, 1)
		assert.Equal(t, compliance.ActionBlock, rule.Actions[0].Type)
	})
	
	t.Run("multiple actions with params", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithActions([]compliance.Action{
				{
					Type:   compliance.ActionWarn,
					Params: map[string]interface{}{"message": "Time restriction warning"},
				},
				{
					Type:   compliance.ActionLog,
					Params: map[string]interface{}{"severity": "medium"},
				},
				{
					Type:   compliance.ActionTimeRestrict,
					Params: map[string]interface{}{"retry_after": "09:00"},
				},
			}).
			Build()
		
		assert.Len(t, rule.Actions, 3)
		assert.NotNil(t, rule.Actions[0].Params)
	})
}

func TestCompliance_Scenarios(t *testing.T) {
	scenarios := fixtures.NewComplianceScenarios(t)
	
	t.Run("TCPA time rule", func(t *testing.T) {
		rule := scenarios.TCPATimeRule()
		assert.Equal(t, compliance.RuleTypeTCPA, rule.Type)
		assert.Equal(t, 1000, rule.Priority)
		assert.NotEmpty(t, rule.TimeWindows)
		assert.NotEmpty(t, rule.Conditions)
		assert.NotEmpty(t, rule.Actions)
	})
	
	t.Run("DNC rule", func(t *testing.T) {
		rule := scenarios.DNCRule()
		assert.Equal(t, compliance.RuleTypeDNC, rule.Type)
		assert.Equal(t, 2000, rule.Priority) // Highest priority
		
		// Should have DNC list check condition
		found := false
		for _, cond := range rule.Conditions {
			if cond.Field == "dnc_list_check" {
				found = true
				break
			}
		}
		assert.True(t, found, "DNC rule should have dnc_list_check condition")
	})
	
	t.Run("state specific rule", func(t *testing.T) {
		rule := scenarios.StateSpecificRule("CA")
		assert.Equal(t, compliance.RuleTypeCustom, rule.Type)
		assert.Contains(t, rule.Geography.States, "CA")
		assert.NotEmpty(t, rule.TimeWindows)
	})
	
	t.Run("GDPR rule", func(t *testing.T) {
		rule := scenarios.GDPRRule()
		assert.Equal(t, compliance.RuleTypeGDPR, rule.Type)
		assert.NotEmpty(t, rule.Geography.Countries)
		assert.Contains(t, rule.Geography.Countries, "GB")
		assert.Contains(t, rule.Geography.Countries, "FR")
		assert.Contains(t, rule.Geography.Countries, "DE")
	})
}

func TestCompliance_ConsentScenarios(t *testing.T) {
	scenarios := fixtures.NewComplianceScenarios(t)
	
	t.Run("express consent", func(t *testing.T) {
		consent := scenarios.ExpressConsent("+15551234567")
		assert.Equal(t, "+15551234567", consent.PhoneNumber)
		assert.Equal(t, compliance.ConsentStatusActive, consent.Status)
		assert.NotNil(t, consent.ExpiresAt)
		assert.True(t, consent.ExpiresAt.After(time.Now()))
	})
	
	t.Run("revoked consent", func(t *testing.T) {
		consent := scenarios.RevokedConsent("+15559876543")
		assert.Equal(t, compliance.ConsentStatusRevoked, consent.Status)
	})
}

func TestCompliance_ViolationScenarios(t *testing.T) {
	scenarios := fixtures.NewComplianceScenarios(t)
	
	callID := uuid.New()
	accountID := uuid.New()
	ruleID := uuid.New()
	
	violation := scenarios.ViolationRecord(callID, accountID, ruleID)
	
	assert.NotEqual(t, uuid.Nil, violation.ID)
	assert.Equal(t, callID, violation.CallID)
	assert.Equal(t, accountID, violation.AccountID)
	assert.Equal(t, ruleID, violation.RuleID)
	assert.Equal(t, compliance.ViolationTCPA, violation.ViolationType)
	assert.Equal(t, compliance.SeverityHigh, violation.Severity)
	assert.False(t, violation.Resolved)
	assert.Nil(t, violation.ResolvedBy)
	assert.Nil(t, violation.ResolvedAt)
}

func TestCompliance_RuleExpiration(t *testing.T) {
	t.Run("rule without expiration", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).Build()
		assert.Nil(t, rule.ExpiresAt)
	})
	
	t.Run("rule with future expiration", func(t *testing.T) {
		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithExpiration(expiresAt).
			Build()
		
		assert.NotNil(t, rule.ExpiresAt)
		assert.Equal(t, expiresAt.Unix(), rule.ExpiresAt.Unix())
	})
	
	t.Run("expired rule", func(t *testing.T) {
		expiresAt := time.Now().Add(-1 * time.Hour)
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithExpiration(expiresAt).
			Build()
		
		assert.True(t, time.Now().After(*rule.ExpiresAt))
	})
}

func TestCompliance_EdgeCases(t *testing.T) {
	t.Run("rule with no conditions", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithConditions([]compliance.Condition{}).
			Build()
		
		assert.Empty(t, rule.Conditions)
	})
	
	t.Run("rule with no actions", func(t *testing.T) {
		rule := fixtures.NewComplianceRuleBuilder(t).
			WithActions([]compliance.Action{}).
			Build()
		
		assert.Empty(t, rule.Actions)
	})
	
	t.Run("consent already revoked", func(t *testing.T) {
		consent := fixtures.NewConsentRecordBuilder(t).
			WithStatus(compliance.ConsentStatusRevoked).
			Build()
		
		optOut := time.Now().Add(-1 * time.Hour)
		consent.OptOutTimestamp = &optOut
		oldStatus := consent.Status
		
		consent.Revoke()
		
		assert.Equal(t, oldStatus, consent.Status) // Still revoked
		assert.NotEqual(t, optOut, *consent.OptOutTimestamp) // Updated timestamp
	})
}

func TestCompliance_Performance(t *testing.T) {
	t.Run("rule creation performance", func(t *testing.T) {
		start := time.Now()
		count := 10000
		
		for i := 0; i < count; i++ {
			_ = compliance.NewComplianceRule("Test Rule", compliance.RuleTypeTCPA, uuid.New())
		}
		
		elapsed := time.Since(start)
		perRule := elapsed / time.Duration(count)
		
		assert.Less(t, perRule, 10*time.Microsecond,
			"Rule creation took %v per rule, expected < 10µs", perRule)
	})
	
	t.Run("consent record creation performance", func(t *testing.T) {
		start := time.Now()
		count := 10000
		
		for i := 0; i < count; i++ {
			_ = compliance.NewConsentRecord("+15551234567", compliance.ConsentTypeExpress, 
				"web", "192.168.1.1", "Mozilla")
		}
		
		elapsed := time.Since(start)
		perConsent := elapsed / time.Duration(count)
		
		assert.Less(t, perConsent, 10*time.Microsecond,
			"Consent creation took %v per record, expected < 10µs", perConsent)
	})
}

// TestCompliance_TableDriven demonstrates table-driven testing pattern
func TestCompliance_TableDriven(t *testing.T) {
	type testCase struct {
		name     string
		setup    func() interface{}
		action   func(interface{}) error
		validate func(*testing.T, interface{}, error)
	}
	
	tests := []testCase{
		{
			name: "consent lifecycle",
			setup: func() interface{} {
				return fixtures.NewConsentRecordBuilder(t).
					WithStatus(compliance.ConsentStatusActive).
					Build()
			},
			action: func(i interface{}) error {
				consent := i.(*compliance.ConsentRecord)
				consent.Revoke()
				return nil
			},
			validate: func(t *testing.T, i interface{}, err error) {
				consent := i.(*compliance.ConsentRecord)
				assert.NoError(t, err)
				assert.Equal(t, compliance.ConsentStatusRevoked, consent.Status)
				assert.NotNil(t, consent.OptOutTimestamp)
			},
		},
		{
			name: "compliance check violations",
			setup: func() interface{} {
				return &compliance.ComplianceCheck{
					CallID:      uuid.New(),
					PhoneNumber: "+15551234567",
					Approved:    true,
					Violations:  []compliance.ComplianceViolation{},
				}
			},
			action: func(i interface{}) error {
				cc := i.(*compliance.ComplianceCheck)
				cc.AddViolation(compliance.ComplianceViolation{
					ID:          uuid.New(),
					Description: "Test violation 1",
				})
				cc.AddViolation(compliance.ComplianceViolation{
					ID:          uuid.New(),
					Description: "Test violation 2",
				})
				return nil
			},
			validate: func(t *testing.T, i interface{}, err error) {
				cc := i.(*compliance.ComplianceCheck)
				assert.NoError(t, err)
				assert.False(t, cc.Approved)
				assert.Len(t, cc.Violations, 2)
				assert.False(t, cc.IsCallAllowed())
			},
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entity := tc.setup()
			err := tc.action(entity)
			tc.validate(t, entity, err)
		})
	}
}