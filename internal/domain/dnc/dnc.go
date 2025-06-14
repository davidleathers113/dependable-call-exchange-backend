// Package dnc provides domain entities and business logic for Do Not Call (DNC) list management.
// It supports multiple DNC providers (Federal, State, Internal) and provides comprehensive
// compliance checking capabilities.
package dnc

// This file serves as the main entry point for the DNC domain.
// All core types are defined in their respective files:
// - entry.go: DNCEntry entity for individual DNC records
// - provider.go: DNCProvider entity for DNC data sources
// - check_result.go: DNCCheckResult entity for aggregated compliance checks