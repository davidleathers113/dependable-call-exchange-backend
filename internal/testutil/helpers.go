package testutil

import (
	"context"
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestContext creates a context with timeout for tests
func TestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// GenerateUUID generates a new UUID for testing
func GenerateUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	return id
}

// AssertEventually asserts that a condition is met within a timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, tick time.Duration, msgAndArgs ...interface{}) {
	t.Helper()
	
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	
	for {
		select {
		case <-timer.C:
			require.FailNow(t, "condition not met within timeout", msgAndArgs...)
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// TimeRange represents a time range for testing
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange creates a new time range for testing
func NewTimeRange(start, end time.Time) TimeRange {
	return TimeRange{Start: start, End: end}
}

// Contains checks if a time is within the range
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// AssertTimeWithin asserts that a time is within an expected range
func AssertTimeWithin(t *testing.T, actual, expected time.Time, delta time.Duration) {
	t.Helper()
	diff := actual.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	require.LessOrEqual(t, diff, delta, 
		"expected time %v to be within %v of %v, but difference was %v",
		actual, delta, expected, diff)
}

// Ptr returns a pointer to the given value (useful for optional fields)
func Ptr[T any](v T) *T {
	return &v
}

// EqualIgnoreOrder checks if two slices contain the same elements regardless of order
func EqualIgnoreOrder[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	
	counts := make(map[T]int)
	for _, item := range a {
		counts[item]++
	}
	
	for _, item := range b {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}
	
	return true
}

// MustParse parses a string into the given type or fails the test
func MustParse[T any](t *testing.T, parser func(string) (T, error), value string) T {
	t.Helper()
	result, err := parser(value)
	require.NoError(t, err, "failed to parse %s", value)
	return result
}

// AssertPanic asserts that a function panics
func AssertPanic(t *testing.T, f func(), msgAndArgs ...interface{}) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			require.FailNow(t, "expected panic but none occurred", msgAndArgs...)
		}
	}()
	f()
}

// AssertNoPanic asserts that a function does not panic
func AssertNoPanic(t *testing.T, f func(), msgAndArgs ...interface{}) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			require.FailNow(t, "unexpected panic", append([]interface{}{r}, msgAndArgs...)...)
		}
	}()
	f()
}