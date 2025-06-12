package call

import "time"

// Clock interface for time operations (supports testing)
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using actual system time
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// MockClock implements Clock for testing
type MockClock struct {
	CurrentTime time.Time
}

func (m *MockClock) Now() time.Time {
	return m.CurrentTime
}

func (m *MockClock) Advance(d time.Duration) {
	m.CurrentTime = m.CurrentTime.Add(d)
}

// Package-level clock variable (defaults to real clock)
var clock Clock = RealClock{}

// SetClock allows tests to inject a mock clock
func SetClock(c Clock) {
	clock = c
}

// ResetClock restores the real clock
func ResetClock() {
	clock = RealClock{}
}
