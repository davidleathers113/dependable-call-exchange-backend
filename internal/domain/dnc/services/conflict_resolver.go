package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/errors"
)

// DNCConflictResolver handles conflicts between different DNC sources and data inconsistencies
type DNCConflictResolver struct {
	repository     dnc.Repository
	sourcePriority map[string]int
	rules          *ConflictResolutionRules
}

// ConflictResolutionRules defines the business rules for resolving conflicts
type ConflictResolutionRules struct {
	// Prefer most recent data when sources conflict
	PreferRecent bool
	
	// Maximum age difference (in hours) to consider data "current"
	MaxDataAge time.Duration
	
	// Whether to merge compatible entries or take highest priority
	MergeCompatible bool
	
	// Trust threshold for automatic resolution (0.0-1.0)
	AutoResolutionThreshold float64
	
	// List type priority order (higher numbers = higher priority)
	ListTypePriority map[types.DNCListType]int
}

// ConflictResult represents the outcome of conflict resolution
type ConflictResult struct {
	ResolvedEntries   []*dnc.Entry
	ConflictsFound    []types.DataConflict
	ResolutionMethod  string
	ConfidenceScore   float64
	RequiresReview    bool
	Warnings          []string
}

// NewDNCConflictResolver creates a new conflict resolver instance
func NewDNCConflictResolver(repository dnc.Repository, sourcePriority map[string]int, rules *ConflictResolutionRules) (*DNCConflictResolver, error) {
	if repository == nil {
		return nil, errors.NewValidationError("INVALID_REPOSITORY", "repository cannot be nil")
	}
	if sourcePriority == nil || len(sourcePriority) == 0 {
		return nil, errors.NewValidationError("INVALID_SOURCE_PRIORITY", "source priority map cannot be empty")
	}
	if rules == nil {
		return nil, errors.NewValidationError("INVALID_RULES", "conflict resolution rules cannot be nil")
	}

	return &DNCConflictResolver{
		repository:     repository,
		sourcePriority: sourcePriority,
		rules:          rules,
	}, nil
}

// ResolveConflicts identifies and resolves conflicts for DNC entries of a specific phone number
func (r *DNCConflictResolver) ResolveConflicts(ctx context.Context, phoneNumber *values.PhoneNumber) (*ConflictResult, error) {
	if phoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "phone number cannot be nil")
	}

	// 1. Retrieve all entries for the phone number
	entries, err := r.repository.FindByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve DNC entries").WithCause(err)
	}

	if len(entries) <= 1 {
		// No conflicts possible with single or no entries
		return &ConflictResult{
			ResolvedEntries:  entries,
			ConflictsFound:   []types.DataConflict{},
			ResolutionMethod: "no_conflicts",
			ConfidenceScore:  1.0,
			RequiresReview:   false,
		}, nil
	}

	// 2. Identify conflicts
	conflicts := r.identifyConflicts(entries)

	// 3. Apply resolution strategies
	resolvedEntries, method, confidence := r.applyResolutionStrategy(entries, conflicts)

	// 4. Validate resolution
	warnings := r.validateResolution(resolvedEntries, conflicts)

	result := &ConflictResult{
		ResolvedEntries:   resolvedEntries,
		ConflictsFound:    conflicts,
		ResolutionMethod:  method,
		ConfidenceScore:   confidence,
		RequiresReview:    confidence < r.rules.AutoResolutionThreshold,
		Warnings:          warnings,
	}

	return result, nil
}

// PrioritizeSources orders data sources by priority and reliability
func (r *DNCConflictResolver) PrioritizeSources(sources []types.DNCSource) []types.DNCSource {
	if len(sources) <= 1 {
		return sources
	}

	// Create a copy to avoid modifying the original slice
	prioritized := make([]types.DNCSource, len(sources))
	copy(prioritized, sources)

	// Sort by priority (higher numbers first)
	sort.Slice(prioritized, func(i, j int) bool {
		priorityI := r.getSourcePriority(prioritized[i].Provider)
		priorityJ := r.getSourcePriority(prioritized[j].Provider)
		
		// Primary sort by priority
		if priorityI != priorityJ {
			return priorityI > priorityJ
		}
		
		// Secondary sort by recency
		return prioritized[i].LastUpdated.After(prioritized[j].LastUpdated)
	})

	return prioritized
}

// MergeResults combines multiple DNC check results into a unified result
func (r *DNCConflictResolver) MergeResults(ctx context.Context, results []*types.DNCCheckResult) (*types.DNCCheckResult, error) {
	if len(results) == 0 {
		return nil, errors.NewValidationError("INVALID_RESULTS", "cannot merge empty results")
	}

	if len(results) == 1 {
		return results[0], nil
	}

	// Initialize merged result with the highest priority result
	prioritizedResults := r.prioritizeResults(results)
	merged := r.copyCheckResult(prioritizedResults[0])

	// Merge data from other results
	for i := 1; i < len(prioritizedResults); i++ {
		result := prioritizedResults[i]
		
		// Merge list memberships
		for listType, membership := range result.ListMemberships {
			if existing, exists := merged.ListMemberships[listType]; exists {
				// Resolve conflict between memberships
				merged.ListMemberships[listType] = r.resolveMembershipConflict(existing, membership)
			} else {
				merged.ListMemberships[listType] = membership
			}
		}
		
		// Update metadata
		merged.Sources = append(merged.Sources, result.Sources...)
		
		// Take the most conservative (blocked) status
		if result.IsBlocked && !merged.IsBlocked {
			merged.IsBlocked = true
			merged.BlockingReason = result.BlockingReason
		}
		
		// Accumulate confidence scores
		merged.ConfidenceScore = r.calculateMergedConfidence(merged.ConfidenceScore, result.ConfidenceScore)
	}

	// Update check metadata
	merged.CheckedAt = time.Now()
	merged.Sources = r.deduplicateSources(merged.Sources)
	merged.Conflicts = r.identifyResultConflicts(results)

	return merged, nil
}

// Helper methods for conflict identification

func (r *DNCConflictResolver) identifyConflicts(entries []*dnc.Entry) []types.DataConflict {
	conflicts := make([]types.DataConflict, 0)

	// Group entries by phone number for comparison
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			entryConflicts := r.compareEntries(entries[i], entries[j])
			conflicts = append(conflicts, entryConflicts...)
		}
	}

	return conflicts
}

func (r *DNCConflictResolver) compareEntries(entry1, entry2 *dnc.Entry) []types.DataConflict {
	conflicts := make([]types.DataConflict, 0)

	// Different sources reporting different list types for same number
	if entry1.ListType != entry2.ListType && entry1.PhoneNumber.String() == entry2.PhoneNumber.String() {
		conflict := types.DataConflict{
			Type:        types.ConflictTypeListMismatch,
			Description: fmt.Sprintf("Sources disagree on list type: %s vs %s", entry1.ListType, entry2.ListType),
			Entry1:      entry1,
			Entry2:      entry2,
			Severity:    types.ConflictSeverityMedium,
		}
		conflicts = append(conflicts, conflict)
	}

	// Conflicting active/inactive status
	if entry1.IsActive() != entry2.IsActive() {
		conflict := types.DataConflict{
			Type:        types.ConflictTypeStatusMismatch,
			Description: "Sources disagree on active status",
			Entry1:      entry1,
			Entry2:      entry2,
			Severity:    types.ConflictSeverityHigh,
		}
		conflicts = append(conflicts, conflict)
	}

	// Significantly different timestamps
	timeDiff := entry1.CreatedAt.Sub(entry2.CreatedAt)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 24*time.Hour && entry1.Source.Provider != entry2.Source.Provider {
		conflict := types.DataConflict{
			Type:        types.ConflictTypeTimeMismatch,
			Description: fmt.Sprintf("Significant time difference: %v", timeDiff),
			Entry1:      entry1,
			Entry2:      entry2,
			Severity:    types.ConflictSeverityLow,
		}
		conflicts = append(conflicts, conflict)
	}

	// State code conflicts for state lists
	if entry1.ListType == types.ListTypeState && entry2.ListType == types.ListTypeState {
		if entry1.StateCode != entry2.StateCode {
			conflict := types.DataConflict{
				Type:        types.ConflictTypeStateMismatch,
				Description: fmt.Sprintf("State code mismatch: %s vs %s", entry1.StateCode, entry2.StateCode),
				Entry1:      entry1,
				Entry2:      entry2,
				Severity:    types.ConflictSeverityHigh,
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// Resolution strategy methods

func (r *DNCConflictResolver) applyResolutionStrategy(entries []*dnc.Entry, conflicts []types.DataConflict) ([]*dnc.Entry, string, float64) {
	if len(conflicts) == 0 {
		return entries, "no_conflicts", 1.0
	}

	// Determine the best resolution strategy based on conflict types
	strategy := r.selectResolutionStrategy(conflicts)

	switch strategy {
	case "priority_based":
		return r.resolvePriorityBased(entries, conflicts)
	case "merge_compatible":
		return r.resolveMergeCompatible(entries, conflicts)
	case "most_recent":
		return r.resolveMostRecent(entries, conflicts)
	case "most_restrictive":
		return r.resolveMostRestrictive(entries, conflicts)
	default:
		return r.resolvePriorityBased(entries, conflicts)
	}
}

func (r *DNCConflictResolver) selectResolutionStrategy(conflicts []types.DataConflict) string {
	// Count conflict types to determine best strategy
	severityCount := make(map[types.ConflictSeverity]int)
	typeCount := make(map[types.ConflictType]int)

	for _, conflict := range conflicts {
		severityCount[conflict.Severity]++
		typeCount[conflict.Type]++
	}

	// High-severity conflicts require priority-based resolution
	if severityCount[types.ConflictSeverityHigh] > 0 {
		return "priority_based"
	}

	// Time mismatches favor most recent
	if typeCount[types.ConflictTypeTimeMismatch] > 0 && r.rules.PreferRecent {
		return "most_recent"
	}

	// Status mismatches require most restrictive approach
	if typeCount[types.ConflictTypeStatusMismatch] > 0 {
		return "most_restrictive"
	}

	// Default to merge if enabled
	if r.rules.MergeCompatible {
		return "merge_compatible"
	}

	return "priority_based"
}

func (r *DNCConflictResolver) resolvePriorityBased(entries []*dnc.Entry, conflicts []types.DataConflict) ([]*dnc.Entry, string, float64) {
	// Sort entries by source priority
	prioritized := make([]*dnc.Entry, len(entries))
	copy(prioritized, entries)

	sort.Slice(prioritized, func(i, j int) bool {
		priorityI := r.getSourcePriority(prioritized[i].Source.Provider)
		priorityJ := r.getSourcePriority(prioritized[j].Source.Provider)
		
		if priorityI != priorityJ {
			return priorityI > priorityJ
		}
		
		// Secondary sort by list type priority
		listPriorityI := r.rules.ListTypePriority[prioritized[i].ListType]
		listPriorityJ := r.rules.ListTypePriority[prioritized[j].ListType]
		return listPriorityI > listPriorityJ
	})

	// Take the highest priority entry for each unique combination
	resolved := make([]*dnc.Entry, 0)
	seen := make(map[string]bool)

	for _, entry := range prioritized {
		key := r.getEntryKey(entry)
		if !seen[key] {
			resolved = append(resolved, entry)
			seen[key] = true
		}
	}

	confidence := r.calculateResolutionConfidence(len(conflicts), len(resolved), "priority_based")
	return resolved, "priority_based", confidence
}

func (r *DNCConflictResolver) resolveMergeCompatible(entries []*dnc.Entry, conflicts []types.DataConflict) ([]*dnc.Entry, string, float64) {
	// Group compatible entries and merge their information
	groups := r.groupCompatibleEntries(entries)
	resolved := make([]*dnc.Entry, 0, len(groups))

	for _, group := range groups {
		if len(group) == 1 {
			resolved = append(resolved, group[0])
		} else {
			merged := r.mergeEntryGroup(group)
			resolved = append(resolved, merged)
		}
	}

	confidence := r.calculateResolutionConfidence(len(conflicts), len(resolved), "merge_compatible")
	return resolved, "merge_compatible", confidence
}

func (r *DNCConflictResolver) resolveMostRecent(entries []*dnc.Entry, conflicts []types.DataConflict) ([]*dnc.Entry, string, float64) {
	// Group entries by logical equivalence and keep most recent
	groups := r.groupEquivalentEntries(entries)
	resolved := make([]*dnc.Entry, 0, len(groups))

	for _, group := range groups {
		// Sort by creation time and take most recent
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreatedAt.After(group[j].CreatedAt)
		})
		resolved = append(resolved, group[0])
	}

	confidence := r.calculateResolutionConfidence(len(conflicts), len(resolved), "most_recent")
	return resolved, "most_recent", confidence
}

func (r *DNCConflictResolver) resolveMostRestrictive(entries []*dnc.Entry, conflicts []types.DataConflict) ([]*dnc.Entry, string, float64) {
	// Take the most restrictive interpretation when in doubt
	resolved := make([]*dnc.Entry, 0)
	
	// Group by phone number and list type
	groups := make(map[string][]*dnc.Entry)
	for _, entry := range entries {
		key := entry.PhoneNumber.String() + ":" + string(entry.ListType)
		groups[key] = append(groups[key], entry)
	}

	for _, group := range groups {
		// Find the most restrictive entry (active > inactive, federal > state > internal)
		mostRestrictive := group[0]
		for _, entry := range group[1:] {
			if r.isMoreRestrictive(entry, mostRestrictive) {
				mostRestrictive = entry
			}
		}
		resolved = append(resolved, mostRestrictive)
	}

	confidence := r.calculateResolutionConfidence(len(conflicts), len(resolved), "most_restrictive")
	return resolved, "most_restrictive", confidence
}

// Helper methods

func (r *DNCConflictResolver) getSourcePriority(provider string) int {
	if priority, exists := r.sourcePriority[provider]; exists {
		return priority
	}
	return 0 // Default priority for unknown sources
}

func (r *DNCConflictResolver) getEntryKey(entry *dnc.Entry) string {
	return fmt.Sprintf("%s:%s", entry.PhoneNumber.String(), entry.ListType)
}

func (r *DNCConflictResolver) groupCompatibleEntries(entries []*dnc.Entry) [][]*dnc.Entry {
	groups := make([][]*dnc.Entry, 0)
	used := make(map[int]bool)

	for i, entry := range entries {
		if used[i] {
			continue
		}

		group := []*dnc.Entry{entry}
		used[i] = true

		// Find other entries compatible with this one
		for j := i + 1; j < len(entries); j++ {
			if used[j] {
				continue
			}

			if r.areCompatible(entry, entries[j]) {
				group = append(group, entries[j])
				used[j] = true
			}
		}

		groups = append(groups, group)
	}

	return groups
}

func (r *DNCConflictResolver) groupEquivalentEntries(entries []*dnc.Entry) [][]*dnc.Entry {
	groups := make(map[string][]*dnc.Entry)

	for _, entry := range entries {
		key := entry.PhoneNumber.String() + ":" + string(entry.ListType)
		groups[key] = append(groups[key], entry)
	}

	result := make([][]*dnc.Entry, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}

	return result
}

func (r *DNCConflictResolver) areCompatible(entry1, entry2 *dnc.Entry) bool {
	// Entries are compatible if they don't contradict each other
	if entry1.PhoneNumber.String() != entry2.PhoneNumber.String() {
		return false
	}

	// Same list type entries are always compatible
	if entry1.ListType == entry2.ListType {
		return true
	}

	// Different active status makes them incompatible
	if entry1.IsActive() != entry2.IsActive() {
		return false
	}

	// Federal and state listings can coexist
	if (entry1.ListType == types.ListTypeFederal && entry2.ListType == types.ListTypeState) ||
		(entry1.ListType == types.ListTypeState && entry2.ListType == types.ListTypeFederal) {
		return true
	}

	return false
}

func (r *DNCConflictResolver) isMoreRestrictive(entry1, entry2 *dnc.Entry) bool {
	// Active is more restrictive than inactive
	if entry1.IsActive() && !entry2.IsActive() {
		return true
	}
	if !entry1.IsActive() && entry2.IsActive() {
		return false
	}

	// Compare list type restrictiveness
	priority1 := r.rules.ListTypePriority[entry1.ListType]
	priority2 := r.rules.ListTypePriority[entry2.ListType]
	return priority1 > priority2
}

func (r *DNCConflictResolver) mergeEntryGroup(group []*dnc.Entry) *dnc.Entry {
	if len(group) == 0 {
		return nil
	}

	// Start with the highest priority entry
	base := group[0]
	for _, entry := range group[1:] {
		if r.getSourcePriority(entry.Source.Provider) > r.getSourcePriority(base.Source.Provider) {
			base = entry
		}
	}

	// Create merged entry
	merged := &dnc.Entry{
		PhoneNumber: base.PhoneNumber,
		ListType:    base.ListType,
		StateCode:   base.StateCode,
		Source:      base.Source,
		CreatedAt:   base.CreatedAt,
		ExpiresAt:   base.ExpiresAt,
		Metadata:    make(map[string]interface{}),
	}

	// Merge metadata from all entries
	for _, entry := range group {
		for key, value := range entry.Metadata {
			merged.Metadata[key] = value
		}
	}

	// Add merge information
	merged.Metadata["merged_from"] = len(group)
	merged.Metadata["merge_sources"] = r.extractSourceNames(group)

	return merged
}

func (r *DNCConflictResolver) extractSourceNames(entries []*dnc.Entry) []string {
	sources := make([]string, 0, len(entries))
	seen := make(map[string]bool)

	for _, entry := range entries {
		if !seen[entry.Source.Provider] {
			sources = append(sources, entry.Source.Provider)
			seen[entry.Source.Provider] = true
		}
	}

	return sources
}

func (r *DNCConflictResolver) calculateResolutionConfidence(conflictCount, resolvedCount int, method string) float64 {
	baseConfidence := 0.9

	// Reduce confidence based on number of conflicts
	conflictPenalty := float64(conflictCount) * 0.1
	
	// Adjust for resolution method
	methodBonus := 0.0
	switch method {
	case "priority_based":
		methodBonus = 0.1
	case "most_restrictive":
		methodBonus = 0.05
	case "merge_compatible":
		methodBonus = 0.0
	case "most_recent":
		methodBonus = -0.05
	}

	confidence := baseConfidence - conflictPenalty + methodBonus

	// Ensure confidence is between 0.0 and 1.0
	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (r *DNCConflictResolver) validateResolution(resolved []*dnc.Entry, conflicts []types.DataConflict) []string {
	warnings := make([]string, 0)

	// Check for data quality issues
	for _, entry := range resolved {
		if time.Since(entry.CreatedAt) > r.rules.MaxDataAge {
			warnings = append(warnings, fmt.Sprintf("Stale data for %s from %s", entry.PhoneNumber, entry.Source.Provider))
		}

		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
			warnings = append(warnings, fmt.Sprintf("Expired entry for %s", entry.PhoneNumber))
		}
	}

	// Check if high-severity conflicts remain unresolved
	for _, conflict := range conflicts {
		if conflict.Severity == types.ConflictSeverityHigh {
			warnings = append(warnings, fmt.Sprintf("High-severity conflict may require manual review: %s", conflict.Description))
		}
	}

	return warnings
}

// Result merging helper methods

func (r *DNCConflictResolver) prioritizeResults(results []*types.DNCCheckResult) []*types.DNCCheckResult {
	prioritized := make([]*types.DNCCheckResult, len(results))
	copy(prioritized, results)

	sort.Slice(prioritized, func(i, j int) bool {
		// Primary sort by blocked status (blocked results first)
		if prioritized[i].IsBlocked != prioritized[j].IsBlocked {
			return prioritized[i].IsBlocked
		}

		// Secondary sort by confidence score
		return prioritized[i].ConfidenceScore > prioritized[j].ConfidenceScore
	})

	return prioritized
}

func (r *DNCConflictResolver) copyCheckResult(original *types.DNCCheckResult) *types.DNCCheckResult {
	copy := &types.DNCCheckResult{
		PhoneNumber:      original.PhoneNumber,
		IsBlocked:        original.IsBlocked,
		BlockingReason:   original.BlockingReason,
		ListMemberships:  make(map[types.DNCListType]types.ListMembership),
		ConfidenceScore:  original.ConfidenceScore,
		CheckedAt:        original.CheckedAt,
		Sources:          make([]string, len(original.Sources)),
	}

	// Deep copy list memberships
	for listType, membership := range original.ListMemberships {
		copy.ListMemberships[listType] = membership
	}

	// Copy sources
	copy.Sources = append(copy.Sources, original.Sources...)

	return copy
}

func (r *DNCConflictResolver) resolveMembershipConflict(existing, new types.ListMembership) types.ListMembership {
	// Take the more restrictive membership
	if new.IsListed && !existing.IsListed {
		return new
	}
	if existing.IsListed && !new.IsListed {
		return existing
	}

	// If both agree on listing status, merge additional data
	resolved := existing
	if new.ListedDate.After(existing.ListedDate) {
		resolved.ListedDate = new.ListedDate
	}
	if new.Confidence > existing.Confidence {
		resolved.Confidence = new.Confidence
	}

	return resolved
}

func (r *DNCConflictResolver) calculateMergedConfidence(conf1, conf2 float64) float64 {
	// Average confidence with slight penalty for merging
	return (conf1 + conf2) / 2.0 * 0.95
}

func (r *DNCConflictResolver) deduplicateSources(sources []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, source := range sources {
		if !seen[source] {
			result = append(result, source)
			seen[source] = true
		}
	}

	return result
}

func (r *DNCConflictResolver) identifyResultConflicts(results []*types.DNCCheckResult) []string {
	conflicts := make([]string, 0)

	// Check for disagreement on blocked status
	blockedCount := 0
	for _, result := range results {
		if result.IsBlocked {
			blockedCount++
		}
	}

	if blockedCount > 0 && blockedCount < len(results) {
		conflicts = append(conflicts, "Sources disagree on blocked status")
	}

	// Check for significant confidence score differences
	minConf := 1.0
	maxConf := 0.0
	for _, result := range results {
		if result.ConfidenceScore < minConf {
			minConf = result.ConfidenceScore
		}
		if result.ConfidenceScore > maxConf {
			maxConf = result.ConfidenceScore
		}
	}

	if maxConf-minConf > 0.3 {
		conflicts = append(conflicts, "Significant confidence score differences between sources")
	}

	return conflicts
}