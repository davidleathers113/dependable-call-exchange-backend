package mocks

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// BaseMock provides common mock functionality
type BaseMock struct {
	mock.Mock
	t *testing.T
}

// NewBaseMock creates a new BaseMock
func NewBaseMock(t *testing.T) *BaseMock {
	return &BaseMock{t: t}
}

// AssertExpectations asserts that all expectations were met
func (m *BaseMock) AssertExpectations() {
	m.t.Helper()
	m.Mock.AssertExpectations(m.t)
}

// AssertCalled asserts that a method was called
func (m *BaseMock) AssertCalled(methodName string, arguments ...interface{}) {
	m.t.Helper()
	m.Mock.AssertCalled(m.t, methodName, arguments...)
}

// AssertNotCalled asserts that a method was not called
func (m *BaseMock) AssertNotCalled(methodName string, arguments ...interface{}) {
	m.t.Helper()
	m.Mock.AssertNotCalled(m.t, methodName, arguments...)
}

// AssertNumberOfCalls asserts the number of times a method was called
func (m *BaseMock) AssertNumberOfCalls(methodName string, expectedCalls int) {
	m.t.Helper()
	m.Mock.AssertNumberOfCalls(m.t, methodName, expectedCalls)
}

// CallTracker tracks method calls with timing information
type CallTracker struct {
	Method    string
	Arguments []interface{}
	Returns   []interface{}
	CalledAt  time.Time
	Error     error
}

// TrackedMock provides call tracking functionality
type TrackedMock struct {
	*BaseMock
	calls []CallTracker
}

// NewTrackedMock creates a new TrackedMock
func NewTrackedMock(t *testing.T) *TrackedMock {
	return &TrackedMock{
		BaseMock: NewBaseMock(t),
		calls:    make([]CallTracker, 0),
	}
}

// TrackCall records a method call
func (m *TrackedMock) TrackCall(method string, args []interface{}, returns []interface{}) {
	m.calls = append(m.calls, CallTracker{
		Method:    method,
		Arguments: args,
		Returns:   returns,
		CalledAt:  time.Now(),
	})
}

// GetCalls returns all tracked calls
func (m *TrackedMock) GetCalls() []CallTracker {
	return m.calls
}

// GetCallsForMethod returns calls for a specific method
func (m *TrackedMock) GetCallsForMethod(method string) []CallTracker {
	var calls []CallTracker
	for _, call := range m.calls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}

// LastCall returns the most recent call
func (m *TrackedMock) LastCall() *CallTracker {
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

// ClearCalls clears all tracked calls
func (m *TrackedMock) ClearCalls() {
	m.calls = make([]CallTracker, 0)
}

// MatcherFunc is a custom argument matcher function
type MatcherFunc func(interface{}) bool

// Match creates a custom matcher for mock arguments
func Match(fn MatcherFunc) interface{} {
	return mock.MatchedBy(func(arg interface{}) bool {
		return fn(arg)
	})
}

// AnyUUID matches any UUID string
func AnyUUID() interface{} {
	return Match(func(arg interface{}) bool {
		_, ok := arg.(string)
		if !ok {
			return false
		}
		// Simple UUID format check
		str := arg.(string)
		return len(str) == 36 && str[8] == '-' && str[13] == '-' && str[18] == '-' && str[23] == '-'
	})
}

// AnyTime matches any time.Time
func AnyTime() interface{} {
	return Match(func(arg interface{}) bool {
		_, ok := arg.(time.Time)
		return ok
	})
}

// TimeWithin matches a time within a duration of expected
func TimeWithin(expected time.Time, delta time.Duration) interface{} {
	return Match(func(arg interface{}) bool {
		t, ok := arg.(time.Time)
		if !ok {
			return false
		}
		diff := t.Sub(expected)
		if diff < 0 {
			diff = -diff
		}
		return diff <= delta
	})
}

// AnyContext matches any context.Context
func AnyContext() interface{} {
	return mock.AnythingOfType("*context.emptyCtx")
}

// AnythingOfTypes matches any of the specified types
func AnythingOfTypes(types ...string) interface{} {
	return Match(func(arg interface{}) bool {
		// Get the actual type of the argument
		if arg == nil {
			return false
		}
		argType := fmt.Sprintf("%T", arg)
		for _, t := range types {
			if argType == t {
				return true
			}
		}
		return false
	})
}
