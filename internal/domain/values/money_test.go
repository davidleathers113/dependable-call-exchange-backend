package values

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   decimal.Decimal
		currency string
		wantErr  bool
	}{
		{
			name:     "valid USD amount",
			amount:   decimal.NewFromFloat(123.45),
			currency: USD,
			wantErr:  false,
		},
		{
			name:     "valid EUR amount",
			amount:   decimal.NewFromFloat(100.0),
			currency: EUR,
			wantErr:  false,
		},
		{
			name:     "zero amount",
			amount:   decimal.Zero,
			currency: USD,
			wantErr:  false,
		},
		{
			name:     "negative amount",
			amount:   decimal.NewFromFloat(-50.0),
			currency: USD,
			wantErr:  false,
		},
		{
			name:     "empty currency",
			amount:   decimal.NewFromFloat(100.0),
			currency: "",
			wantErr:  true,
		},
		{
			name:     "invalid currency",
			amount:   decimal.NewFromFloat(100.0),
			currency: "INVALID",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoney(tt.amount, tt.currency)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			assert.True(t, money.Amount().Equal(tt.amount))
			assert.Equal(t, tt.currency, money.Currency())
		})
	}
}

func TestNewMoneyFromString(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid decimal string",
			amount:   "123.45",
			currency: USD,
			expected: "123.45",
			wantErr:  false,
		},
		{
			name:     "integer string",
			amount:   "100",
			currency: EUR,
			expected: "100",
			wantErr:  false,
		},
		{
			name:     "invalid amount string",
			amount:   "not-a-number",
			currency: USD,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoneyFromString(tt.amount, tt.currency)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.expected, money.Amount().String())
			assert.Equal(t, tt.currency, money.Currency())
		})
	}
}

func TestNewMoneyFromCents(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency string
		expected string
	}{
		{
			name:     "positive cents",
			cents:    12345,
			currency: USD,
			expected: "123.45",
		},
		{
			name:     "zero cents",
			cents:    0,
			currency: USD,
			expected: "0",
		},
		{
			name:     "negative cents",
			cents:    -5000,
			currency: EUR,
			expected: "-50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoneyFromCents(tt.cents, tt.currency)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, money.Amount().String())
			assert.Equal(t, tt.currency, money.Currency())
		})
	}
}

func TestMoney_String(t *testing.T) {
	tests := []struct {
		name     string
		money    Money
		expected string
	}{
		{
			name:     "USD amount",
			money:    MustNewMoney(decimal.NewFromFloat(123.45), USD),
			expected: "$123.45",
		},
		{
			name:     "EUR amount",
			money:    MustNewMoney(decimal.NewFromFloat(100.0), EUR),
			expected: "€100.00",
		},
		{
			name:     "zero amount",
			money:    Zero(USD),
			expected: "$0.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.money.String())
		})
	}
}

func TestMoney_ArithmeticOperations(t *testing.T) {
	money1 := MustNewMoney(decimal.NewFromFloat(100.0), USD)
	money2 := MustNewMoney(decimal.NewFromFloat(50.0), USD)
	moneyEUR := MustNewMoney(decimal.NewFromFloat(50.0), EUR)

	t.Run("Add same currency", func(t *testing.T) {
		result, err := money1.Add(money2)
		require.NoError(t, err)
		assert.Equal(t, "150", result.Amount().String())
		assert.Equal(t, USD, result.Currency())
	})

	t.Run("Add different currency", func(t *testing.T) {
		_, err := money1.Add(moneyEUR)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot add different currencies")
	})

	t.Run("Subtract same currency", func(t *testing.T) {
		result, err := money1.Sub(money2)
		require.NoError(t, err)
		assert.Equal(t, "50", result.Amount().String())
		assert.Equal(t, USD, result.Currency())
	})

	t.Run("Multiply by factor", func(t *testing.T) {
		result := money1.Mul(decimal.NewFromFloat(1.5))
		assert.Equal(t, "150", result.Amount().String())
		assert.Equal(t, USD, result.Currency())
	})

	t.Run("Divide by factor", func(t *testing.T) {
		result, err := money1.Div(decimal.NewFromFloat(2.0))
		require.NoError(t, err)
		assert.Equal(t, "50", result.Amount().String())
		assert.Equal(t, USD, result.Currency())
	})

	t.Run("Divide by zero", func(t *testing.T) {
		_, err := money1.Div(decimal.Zero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "division by zero")
	})
}

func TestMoney_Comparison(t *testing.T) {
	money1 := MustNewMoney(decimal.NewFromFloat(100.0), USD)
	money2 := MustNewMoney(decimal.NewFromFloat(100.0), USD)
	money3 := MustNewMoney(decimal.NewFromFloat(50.0), USD)
	moneyEUR := MustNewMoney(decimal.NewFromFloat(100.0), EUR)

	t.Run("Equal same currency", func(t *testing.T) {
		assert.True(t, money1.Equal(money2))
		assert.False(t, money1.Equal(money3))
	})

	t.Run("Equal different currency", func(t *testing.T) {
		assert.False(t, money1.Equal(moneyEUR))
	})

	t.Run("Compare same currency", func(t *testing.T) {
		assert.Equal(t, 0, money1.Compare(money2))
		assert.Equal(t, 1, money1.Compare(money3))
		assert.Equal(t, -1, money3.Compare(money1))
	})

	t.Run("Compare different currency panics", func(t *testing.T) {
		assert.Panics(t, func() {
			money1.Compare(moneyEUR)
		})
	})
}

func TestMoney_Properties(t *testing.T) {
	positive := MustNewMoney(decimal.NewFromFloat(100.0), USD)
	negative := MustNewMoney(decimal.NewFromFloat(-50.0), USD)
	zero := Zero(USD)

	t.Run("IsZero", func(t *testing.T) {
		assert.True(t, zero.IsZero())
		assert.False(t, positive.IsZero())
		assert.False(t, negative.IsZero())
	})

	t.Run("IsPositive", func(t *testing.T) {
		assert.True(t, positive.IsPositive())
		assert.False(t, zero.IsPositive())
		assert.False(t, negative.IsPositive())
	})

	t.Run("IsNegative", func(t *testing.T) {
		assert.True(t, negative.IsNegative())
		assert.False(t, zero.IsNegative())
		assert.False(t, positive.IsNegative())
	})
}

func TestMoney_Rounding(t *testing.T) {
	dec, _ := decimal.NewFromString("123.456789")
	money := MustNewMoney(dec, USD)

	t.Run("Round to 2 places", func(t *testing.T) {
		rounded := money.Round(2)
		assert.Equal(t, "123.46", rounded.Amount().String())
	})

	t.Run("Round to nearest cent", func(t *testing.T) {
		rounded := money.RoundToNearestCent()
		assert.Equal(t, "123.46", rounded.Amount().String())
	})
}

func TestMoney_Conversion(t *testing.T) {
	money := MustNewMoney(decimal.NewFromFloat(123.45), USD)

	t.Run("ToCents", func(t *testing.T) {
		cents := money.ToCents()
		assert.Equal(t, int64(12345), cents)
	})

	t.Run("ToFloat64", func(t *testing.T) {
		f := money.ToFloat64()
		assert.InDelta(t, 123.45, f, 0.001)
	})
}

func TestMoney_JSON(t *testing.T) {
	money := MustNewMoney(decimal.NewFromFloat(123.45), USD)

	t.Run("Marshal", func(t *testing.T) {
		data, err := json.Marshal(money)
		require.NoError(t, err)
		
		expected := `{"amount":"123.45","currency":"USD"}`
		assert.JSONEq(t, expected, string(data))
	})

	t.Run("Unmarshal", func(t *testing.T) {
		data := `{"amount":"123.45","currency":"USD"}`
		
		var money Money
		err := json.Unmarshal([]byte(data), &money)
		require.NoError(t, err)
		
		assert.Equal(t, "123.45", money.Amount().String())
		assert.Equal(t, USD, money.Currency())
	})

	t.Run("Unmarshal invalid amount", func(t *testing.T) {
		data := `{"amount":"invalid","currency":"USD"}`
		
		var money Money
		err := json.Unmarshal([]byte(data), &money)
		assert.Error(t, err)
	})
}

func TestMoney_Database(t *testing.T) {
	money := MustNewMoney(decimal.NewFromFloat(123.45), USD)

	t.Run("Value", func(t *testing.T) {
		value, err := money.Value()
		require.NoError(t, err)
		
		// Should return JSON bytes
		assert.NotNil(t, value)
	})

	t.Run("Scan from string", func(t *testing.T) {
		var money Money
		err := money.Scan("123.45")
		require.NoError(t, err)
		
		assert.Equal(t, "123.45", money.Amount().String())
		assert.Equal(t, USD, money.Currency()) // Default currency
	})

	t.Run("Scan from JSON", func(t *testing.T) {
		var money Money
		jsonData := `{"amount":"123.45","currency":"EUR"}`
		err := money.Scan(jsonData)
		require.NoError(t, err)
		
		assert.Equal(t, "123.45", money.Amount().String())
		assert.Equal(t, EUR, money.Currency())
	})

	t.Run("Scan nil", func(t *testing.T) {
		var money Money
		err := money.Scan(nil)
		require.NoError(t, err)
		
		assert.True(t, money.IsZero())
		assert.Equal(t, "", money.Currency())
	})
}

func TestZero(t *testing.T) {
	zero := Zero(USD)
	assert.True(t, zero.IsZero())
	assert.Equal(t, USD, zero.Currency())
	assert.Equal(t, "$0.00", zero.String())
}

func TestMustNewMoney(t *testing.T) {
	t.Run("Valid money", func(t *testing.T) {
		money := MustNewMoney(decimal.NewFromFloat(100.0), USD)
		assert.Equal(t, "100", money.Amount().String())
		assert.Equal(t, USD, money.Currency())
	})

	t.Run("Invalid currency panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNewMoney(decimal.NewFromFloat(100.0), "INVALID")
		})
	})
}