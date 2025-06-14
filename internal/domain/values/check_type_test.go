package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCheckType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid manual check",
			input:     "manual",
			wantValue: "manual",
			wantErr:   false,
		},
		{
			name:      "valid automated check",
			input:     "automated",
			wantValue: "automated",
			wantErr:   false,
		},
		{
			name:      "valid periodic check",
			input:     "periodic",
			wantValue: "periodic",
			wantErr:   false,
		},
		{
			name:      "valid real_time check",
			input:     "real_time",
			wantValue: "real_time",
			wantErr:   false,
		},
		{
			name:      "uppercase input",
			input:     "MANUAL",
			wantValue: "manual",
			wantErr:   false,
		},
		{
			name:      "with spaces converted to underscores",
			input:     "real time",
			wantValue: "real_time",
			wantErr:   false,
		},
		{
			name:      "with hyphens converted to underscores",
			input:     "real-time",
			wantValue: "real_time",
			wantErr:   false,
		},
		{
			name:      "with mixed separators",
			input:     "REAL-TIME",
			wantValue: "real_time",
			wantErr:   false,
		},
		{
			name:    "empty check type",
			input:   "",
			wantErr: true,
			errCode: "EMPTY_CHECK_TYPE",
		},
		{
			name:    "invalid check type",
			input:   "invalid",
			wantErr: true,
			errCode: "UNSUPPORTED_CHECK_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := NewCheckType(tt.input)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, ct.Value())
			assert.Equal(t, tt.wantValue, ct.String())
			assert.True(t, ct.IsValid())
			assert.False(t, ct.IsEmpty())
		})
	}
}

func TestCheckTypeMethods(t *testing.T) {
	t.Run("standard constructors", func(t *testing.T) {
		manual := ManualCheckType()
		assert.Equal(t, "manual", manual.Value())
		assert.True(t, manual.IsManual())
		assert.False(t, manual.IsAutomated())
		
		automated := AutomatedCheckType()
		assert.Equal(t, "automated", automated.Value())
		assert.True(t, automated.IsAutomated())
		assert.False(t, automated.IsPeriodic())
		
		periodic := PeriodicCheckType()
		assert.Equal(t, "periodic", periodic.Value())
		assert.True(t, periodic.IsPeriodic())
		assert.False(t, periodic.IsRealTime())
		
		realTime := RealTimeCheckType()
		assert.Equal(t, "real_time", realTime.Value())
		assert.True(t, realTime.IsRealTime())
		assert.False(t, realTime.IsManual())
	})

	t.Run("display names", func(t *testing.T) {
		assert.Equal(t, "Manual Check", ManualCheckType().DisplayName())
		assert.Equal(t, "Automated Check", AutomatedCheckType().DisplayName())
		assert.Equal(t, "Periodic Check", PeriodicCheckType().DisplayName())
		assert.Equal(t, "Real-Time Check", RealTimeCheckType().DisplayName())
	})

	t.Run("performance characteristics", func(t *testing.T) {
		manual := ManualCheckType()
		automated := AutomatedCheckType()
		periodic := PeriodicCheckType()
		realTime := RealTimeCheckType()
		
		// Latency
		assert.Equal(t, 0, manual.ExpectedLatencyMs())
		assert.Equal(t, 100, automated.ExpectedLatencyMs())
		assert.Equal(t, 0, periodic.ExpectedLatencyMs())
		assert.Equal(t, 50, realTime.ExpectedLatencyMs())
		
		// Cost
		assert.Equal(t, 1, manual.RelativeCost())
		assert.Equal(t, 3, automated.RelativeCost())
		assert.Equal(t, 2, periodic.RelativeCost())
		assert.Equal(t, 5, realTime.RelativeCost())
		
		// API requirements
		assert.False(t, manual.RequiresAPI())
		assert.True(t, automated.RequiresAPI())
		assert.False(t, periodic.RequiresAPI())
		assert.True(t, realTime.RequiresAPI())
	})

	t.Run("audit and cache", func(t *testing.T) {
		manual := ManualCheckType()
		automated := AutomatedCheckType()
		periodic := PeriodicCheckType()
		realTime := RealTimeCheckType()
		
		// All should be auditable
		assert.True(t, manual.IsAuditable())
		assert.True(t, automated.IsAuditable())
		assert.True(t, periodic.IsAuditable())
		assert.True(t, realTime.IsAuditable())
		
		// Cacheable
		assert.False(t, manual.IsCacheable())
		assert.True(t, automated.IsCacheable())
		assert.True(t, periodic.IsCacheable())
		assert.True(t, realTime.IsCacheable())
	})

	t.Run("processing characteristics", func(t *testing.T) {
		manual := ManualCheckType()
		automated := AutomatedCheckType()
		periodic := PeriodicCheckType()
		realTime := RealTimeCheckType()
		
		// User interaction
		assert.True(t, manual.RequiresUserInteraction())
		assert.False(t, automated.RequiresUserInteraction())
		assert.False(t, periodic.RequiresUserInteraction())
		assert.False(t, realTime.RequiresUserInteraction())
		
		// Asynchronous processing
		assert.False(t, manual.IsAsynchronous())
		assert.True(t, automated.IsAsynchronous())
		assert.True(t, periodic.IsAsynchronous())
		assert.False(t, realTime.IsAsynchronous())
	})

	t.Run("cache TTL", func(t *testing.T) {
		assert.Equal(t, 0, ManualCheckType().GetCacheTTL())
		assert.Equal(t, 3600, AutomatedCheckType().GetCacheTTL())
		assert.Equal(t, 86400, PeriodicCheckType().GetCacheTTL())
		assert.Equal(t, 300, RealTimeCheckType().GetCacheTTL())
	})

	t.Run("audit levels", func(t *testing.T) {
		assert.Equal(t, "detailed", ManualCheckType().GetAuditLevel())
		assert.Equal(t, "summary", AutomatedCheckType().GetAuditLevel())
		assert.Equal(t, "summary", PeriodicCheckType().GetAuditLevel())
		assert.Equal(t, "standard", RealTimeCheckType().GetAuditLevel())
	})

	t.Run("processing modes", func(t *testing.T) {
		assert.Equal(t, "interactive", ManualCheckType().GetProcessingMode())
		assert.Equal(t, "asynchronous", AutomatedCheckType().GetProcessingMode())
		assert.Equal(t, "asynchronous", PeriodicCheckType().GetProcessingMode())
		assert.Equal(t, "synchronous", RealTimeCheckType().GetProcessingMode())
	})
}

func TestCheckTypeValidateForContext(t *testing.T) {
	tests := []struct {
		name     string
		checkType CheckType
		context  string
		wantErr  bool
	}{
		// Call initiation context
		{
			name:      "real-time for call initiation - valid",
			checkType: RealTimeCheckType(),
			context:   "call_initiation",
			wantErr:   false,
		},
		{
			name:      "manual for call initiation - invalid",
			checkType: ManualCheckType(),
			context:   "call_initiation",
			wantErr:   true,
		},
		{
			name:      "automated for call initiation - invalid",
			checkType: AutomatedCheckType(),
			context:   "call_initiation",
			wantErr:   true,
		},
		// Batch import context
		{
			name:      "automated for batch import - valid",
			checkType: AutomatedCheckType(),
			context:   "batch_import",
			wantErr:   false,
		},
		{
			name:      "periodic for batch import - valid",
			checkType: PeriodicCheckType(),
			context:   "batch_import",
			wantErr:   false,
		},
		{
			name:      "real-time for batch import - invalid",
			checkType: RealTimeCheckType(),
			context:   "batch_import",
			wantErr:   true,
		},
		{
			name:      "manual for batch import - invalid",
			checkType: ManualCheckType(),
			context:   "batch_import",
			wantErr:   true,
		},
		// Compliance audit context
		{
			name:      "any type for compliance audit - valid",
			checkType: ManualCheckType(),
			context:   "compliance_audit",
			wantErr:   false,
		},
		// Unknown context
		{
			name:      "any type for unknown context - valid",
			checkType: ManualCheckType(),
			context:   "unknown",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.checkType.ValidateForContext(tt.context)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckTypeEqual(t *testing.T) {
	manual1 := ManualCheckType()
	manual2 := ManualCheckType()
	automated := AutomatedCheckType()
	
	assert.True(t, manual1.Equal(manual2))
	assert.False(t, manual1.Equal(automated))
}

func TestCheckTypeJSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		ct := RealTimeCheckType()
		data, err := json.Marshal(ct)
		require.NoError(t, err)
		assert.Equal(t, `"real_time"`, string(data))
	})

	t.Run("unmarshal valid", func(t *testing.T) {
		var ct CheckType
		err := json.Unmarshal([]byte(`"automated"`), &ct)
		require.NoError(t, err)
		assert.Equal(t, "automated", ct.Value())
		assert.True(t, ct.IsAutomated())
	})

	t.Run("unmarshal with conversion", func(t *testing.T) {
		var ct CheckType
		err := json.Unmarshal([]byte(`"real-time"`), &ct)
		require.NoError(t, err)
		assert.Equal(t, "real_time", ct.Value())
		assert.True(t, ct.IsRealTime())
	})

	t.Run("unmarshal invalid", func(t *testing.T) {
		var ct CheckType
		err := json.Unmarshal([]byte(`"invalid"`), &ct)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNSUPPORTED_CHECK_TYPE")
	})
}

func TestCheckTypeDatabase(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		ct := AutomatedCheckType()
		val, err := ct.Value()
		require.NoError(t, err)
		assert.Equal(t, "automated", val)
		
		// Empty check type
		empty := CheckType{}
		val, err = empty.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("scan string", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan("periodic")
		require.NoError(t, err)
		assert.Equal(t, "periodic", ct.Value())
	})

	t.Run("scan with conversion", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan("real-time")
		require.NoError(t, err)
		assert.Equal(t, "real_time", ct.Value())
	})

	t.Run("scan bytes", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan([]byte("manual"))
		require.NoError(t, err)
		assert.Equal(t, "manual", ct.Value())
	})

	t.Run("scan nil", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan(nil)
		require.NoError(t, err)
		assert.True(t, ct.IsEmpty())
	})

	t.Run("scan empty string", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan("")
		require.NoError(t, err)
		assert.True(t, ct.IsEmpty())
	})

	t.Run("scan invalid type", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan(123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot scan")
	})

	t.Run("scan invalid value", func(t *testing.T) {
		var ct CheckType
		err := ct.Scan("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNSUPPORTED_CHECK_TYPE")
	})
}

func TestGetSupportedCheckTypes(t *testing.T) {
	types := GetSupportedCheckTypes()
	assert.Len(t, types, 4)
	
	// Convert to map for easier checking
	typeMap := make(map[string]bool)
	for _, typ := range types {
		typeMap[typ] = true
	}
	
	assert.True(t, typeMap["manual"])
	assert.True(t, typeMap["automated"])
	assert.True(t, typeMap["periodic"])
	assert.True(t, typeMap["real_time"])
}

func TestGetCheckTypeDisplayNames(t *testing.T) {
	names := GetCheckTypeDisplayNames()
	assert.Len(t, names, 4)
	
	// Should contain all display names
	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	
	assert.True(t, nameMap["Manual Check"])
	assert.True(t, nameMap["Automated Check"])
	assert.True(t, nameMap["Periodic Check"])
	assert.True(t, nameMap["Real-Time Check"])
}

func TestValidateCheckType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid check type",
			input:   "manual",
			wantErr: false,
		},
		{
			name:    "valid with underscore",
			input:   "real_time",
			wantErr: false,
		},
		{
			name:    "valid with hyphen",
			input:   "real-time",
			wantErr: false,
		},
		{
			name:    "valid with space",
			input:   "real time",
			wantErr: false,
		},
		{
			name:    "empty check type",
			input:   "",
			wantErr: true,
			errCode: "EMPTY_CHECK_TYPE",
		},
		{
			name:    "invalid check type",
			input:   "unknown",
			wantErr: true,
			errCode: "UNSUPPORTED_CHECK_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCheckType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetOptimalCheckType(t *testing.T) {
	tests := []struct {
		name         string
		useCase      string
		latencyReqMs int
		wantType     string
	}{
		{
			name:         "call routing with low latency",
			useCase:      "call_routing",
			latencyReqMs: 50,
			wantType:     "real_time",
		},
		{
			name:         "call routing with high latency tolerance",
			useCase:      "call_routing",
			latencyReqMs: 200,
			wantType:     "periodic",
		},
		{
			name:         "bulk validation",
			useCase:      "bulk_validation",
			latencyReqMs: 1000,
			wantType:     "automated",
		},
		{
			name:         "user request",
			useCase:      "user_request",
			latencyReqMs: 0,
			wantType:     "manual",
		},
		{
			name:         "scheduled compliance",
			useCase:      "scheduled_compliance",
			latencyReqMs: 5000,
			wantType:     "periodic",
		},
		{
			name:         "unknown use case",
			useCase:      "unknown",
			latencyReqMs: 100,
			wantType:     "automated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := GetOptimalCheckType(tt.useCase, tt.latencyReqMs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, ct.Value())
		})
	}
}

func TestMustNewCheckType(t *testing.T) {
	t.Run("valid check type", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ct := MustNewCheckType("manual")
			assert.Equal(t, "manual", ct.Value())
		})
	})

	t.Run("invalid check type panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNewCheckType("invalid")
		})
	})
}