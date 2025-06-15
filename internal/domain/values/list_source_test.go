package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewListSource(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid federal source",
			input:     "federal",
			wantValue: "federal",
			wantErr:   false,
		},
		{
			name:      "valid state source",
			input:     "state",
			wantValue: "state",
			wantErr:   false,
		},
		{
			name:      "valid internal source",
			input:     "internal",
			wantValue: "internal",
			wantErr:   false,
		},
		{
			name:      "valid custom source",
			input:     "custom",
			wantValue: "custom",
			wantErr:   false,
		},
		{
			name:      "uppercase input",
			input:     "FEDERAL",
			wantValue: "federal",
			wantErr:   false,
		},
		{
			name:      "with spaces",
			input:     " state ",
			wantValue: "state",
			wantErr:   false,
		},
		{
			name:    "empty source",
			input:   "",
			wantErr: true,
			errCode: "EMPTY_LIST_SOURCE",
		},
		{
			name:    "invalid source",
			input:   "invalid",
			wantErr: true,
			errCode: "UNSUPPORTED_LIST_SOURCE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls, err := NewListSource(tt.input)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, ls.Value())
			assert.Equal(t, tt.wantValue, ls.String())
			assert.True(t, ls.IsValid())
			assert.False(t, ls.IsEmpty())
		})
	}
}

func TestListSourceMethods(t *testing.T) {
	t.Run("standard constructors", func(t *testing.T) {
		federal := FederalListSource()
		assert.Equal(t, "federal", federal.Value())
		assert.True(t, federal.IsFederal())
		assert.False(t, federal.IsState())
		
		state := StateListSource()
		assert.Equal(t, "state", state.Value())
		assert.True(t, state.IsState())
		assert.False(t, state.IsFederal())
		
		internal := InternalListSource()
		assert.Equal(t, "internal", internal.Value())
		assert.True(t, internal.IsInternal())
		assert.False(t, internal.IsCustom())
		
		custom := CustomListSource()
		assert.Equal(t, "custom", custom.Value())
		assert.True(t, custom.IsCustom())
		assert.False(t, custom.IsInternal())
	})

	t.Run("display names", func(t *testing.T) {
		assert.Equal(t, "Federal DNC Registry", FederalListSource().DisplayName())
		assert.Equal(t, "State DNC Registry", StateListSource().DisplayName())
		assert.Equal(t, "Internal DNC List", InternalListSource().DisplayName())
		assert.Equal(t, "Custom DNC List", CustomListSource().DisplayName())
	})

	t.Run("authority levels", func(t *testing.T) {
		federal := FederalListSource()
		state := StateListSource()
		internal := InternalListSource()
		custom := CustomListSource()
		
		assert.Equal(t, 4, federal.AuthorityLevel())
		assert.Equal(t, 3, state.AuthorityLevel())
		assert.Equal(t, 2, internal.AuthorityLevel())
		assert.Equal(t, 1, custom.AuthorityLevel())
		
		assert.True(t, federal.HasHigherAuthority(state))
		assert.True(t, state.HasHigherAuthority(internal))
		assert.True(t, internal.HasHigherAuthority(custom))
		assert.False(t, custom.HasHigherAuthority(federal))
	})

	t.Run("regulatory checks", func(t *testing.T) {
		federal := FederalListSource()
		state := StateListSource()
		internal := InternalListSource()
		custom := CustomListSource()
		
		assert.True(t, federal.IsRegulatory())
		assert.True(t, state.IsRegulatory())
		assert.False(t, internal.IsRegulatory())
		assert.False(t, custom.IsRegulatory())
		
		assert.True(t, federal.RequiresCompliance())
		assert.True(t, state.RequiresCompliance())
		assert.False(t, internal.RequiresCompliance())
		assert.False(t, custom.RequiresCompliance())
	})

	t.Run("user modifiable", func(t *testing.T) {
		federal := FederalListSource()
		state := StateListSource()
		internal := InternalListSource()
		custom := CustomListSource()
		
		assert.False(t, federal.IsUserModifiable())
		assert.False(t, state.IsUserModifiable())
		assert.True(t, internal.IsUserModifiable())
		assert.True(t, custom.IsUserModifiable())
	})

	t.Run("compliance codes", func(t *testing.T) {
		assert.Equal(t, "FEDERAL_DNC", FederalListSource().GetComplianceCode())
		assert.Equal(t, "STATE_DNC", StateListSource().GetComplianceCode())
		assert.Equal(t, "", InternalListSource().GetComplianceCode())
		assert.Equal(t, "", CustomListSource().GetComplianceCode())
	})

	t.Run("refresh policies", func(t *testing.T) {
		assert.Equal(t, "monthly", FederalListSource().GetRefreshPolicy())
		assert.Equal(t, "weekly", StateListSource().GetRefreshPolicy())
		assert.Equal(t, "on-demand", InternalListSource().GetRefreshPolicy())
		assert.Equal(t, "on-demand", CustomListSource().GetRefreshPolicy())
	})

	t.Run("priorities", func(t *testing.T) {
		assert.Equal(t, 4, FederalListSource().GetPriority())
		assert.Equal(t, 3, StateListSource().GetPriority())
		assert.Equal(t, 2, InternalListSource().GetPriority())
		assert.Equal(t, 1, CustomListSource().GetPriority())
	})
}

func TestListSourceValidateForOperation(t *testing.T) {
	tests := []struct {
		name      string
		source    ListSource
		operation string
		wantErr   bool
	}{
		// Import operations
		{
			name:      "import to internal - allowed",
			source:    InternalListSource(),
			operation: "import",
			wantErr:   false,
		},
		{
			name:      "import to custom - allowed",
			source:    CustomListSource(),
			operation: "import",
			wantErr:   false,
		},
		{
			name:      "import to federal - not allowed",
			source:    FederalListSource(),
			operation: "import",
			wantErr:   true,
		},
		{
			name:      "import to state - not allowed",
			source:    StateListSource(),
			operation: "import",
			wantErr:   true,
		},
		// Export operations
		{
			name:      "export federal - allowed",
			source:    FederalListSource(),
			operation: "export",
			wantErr:   false,
		},
		{
			name:      "export custom - allowed",
			source:    CustomListSource(),
			operation: "export",
			wantErr:   false,
		},
		// Modify operations
		{
			name:      "modify internal - allowed",
			source:    InternalListSource(),
			operation: "modify",
			wantErr:   false,
		},
		{
			name:      "modify federal - not allowed",
			source:    FederalListSource(),
			operation: "modify",
			wantErr:   true,
		},
		// Query operations
		{
			name:      "query federal - allowed",
			source:    FederalListSource(),
			operation: "query",
			wantErr:   false,
		},
		{
			name:      "query custom - allowed",
			source:    CustomListSource(),
			operation: "query",
			wantErr:   false,
		},
		// Unknown operation
		{
			name:      "unknown operation",
			source:    FederalListSource(),
			operation: "unknown",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.ValidateForOperation(tt.operation)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListSourceEqual(t *testing.T) {
	federal1 := FederalListSource()
	federal2 := FederalListSource()
	state := StateListSource()
	
	assert.True(t, federal1.Equal(federal2))
	assert.False(t, federal1.Equal(state))
}

func TestListSourceJSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		ls := FederalListSource()
		data, err := json.Marshal(ls)
		require.NoError(t, err)
		assert.Equal(t, `"federal"`, string(data))
	})

	t.Run("unmarshal valid", func(t *testing.T) {
		var ls ListSource
		err := json.Unmarshal([]byte(`"state"`), &ls)
		require.NoError(t, err)
		assert.Equal(t, "state", ls.Value())
		assert.True(t, ls.IsState())
	})

	t.Run("unmarshal invalid", func(t *testing.T) {
		var ls ListSource
		err := json.Unmarshal([]byte(`"invalid"`), &ls)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNSUPPORTED_LIST_SOURCE")
	})
}

func TestListSourceDatabase(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		ls := FederalListSource()
		val, err := ls.Value()
		require.NoError(t, err)
		assert.Equal(t, "federal", val)
		
		// Empty source
		empty := ListSource{}
		val, err = empty.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("scan string", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan("state")
		require.NoError(t, err)
		assert.Equal(t, "state", ls.Value())
	})

	t.Run("scan bytes", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan([]byte("internal"))
		require.NoError(t, err)
		assert.Equal(t, "internal", ls.Value())
	})

	t.Run("scan nil", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan(nil)
		require.NoError(t, err)
		assert.True(t, ls.IsEmpty())
	})

	t.Run("scan empty string", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan("")
		require.NoError(t, err)
		assert.True(t, ls.IsEmpty())
	})

	t.Run("scan invalid type", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan(123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot scan")
	})

	t.Run("scan invalid value", func(t *testing.T) {
		var ls ListSource
		err := ls.Scan("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNSUPPORTED_LIST_SOURCE")
	})
}

func TestGetSupportedSources(t *testing.T) {
	sources := GetSupportedSources()
	assert.Len(t, sources, 4)
	
	// Convert to map for easier checking
	sourceMap := make(map[string]bool)
	for _, s := range sources {
		sourceMap[s] = true
	}
	
	assert.True(t, sourceMap["federal"])
	assert.True(t, sourceMap["state"])
	assert.True(t, sourceMap["internal"])
	assert.True(t, sourceMap["custom"])
}

func TestGetRegulatorySourceNames(t *testing.T) {
	names := GetRegulatorySourceNames()
	assert.Len(t, names, 2)
	
	// Should contain display names for regulatory sources
	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	
	assert.True(t, nameMap["Federal DNC Registry"])
	assert.True(t, nameMap["State DNC Registry"])
}

func TestValidateListSource(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid source",
			input:   "federal",
			wantErr: false,
		},
		{
			name:    "valid source uppercase",
			input:   "STATE",
			wantErr: false,
		},
		{
			name:    "empty source",
			input:   "",
			wantErr: true,
			errCode: "EMPTY_LIST_SOURCE",
		},
		{
			name:    "invalid source",
			input:   "unknown",
			wantErr: true,
			errCode: "UNSUPPORTED_LIST_SOURCE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateListSource(tt.input)
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

func TestMustNewListSource(t *testing.T) {
	t.Run("valid source", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ls := MustNewListSource("federal")
			assert.Equal(t, "federal", ls.Value())
		})
	})

	t.Run("invalid source panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNewListSource("invalid")
		})
	})
}