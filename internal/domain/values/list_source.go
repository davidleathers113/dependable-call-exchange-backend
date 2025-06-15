package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ListSource represents a source of DNC (Do Not Call) list data
type ListSource struct {
	source string
}

// Supported list sources
const (
	ListSourceFederal = "federal"
	ListSourceState   = "state"
	ListSourceInternal = "internal"
	ListSourceCustom  = "custom"
)

var (
	// Map of source to display names
	sourceDisplayNames = map[string]string{
		ListSourceFederal:  "Federal DNC Registry",
		ListSourceState:    "State DNC Registry",
		ListSourceInternal: "Internal DNC List",
		ListSourceCustom:   "Custom DNC List",
	}

	// Supported sources for validation
	supportedSources = map[string]bool{
		ListSourceFederal:  true,
		ListSourceState:    true,
		ListSourceInternal: true,
		ListSourceCustom:   true,
	}

	// Authority levels (higher number = higher authority)
	sourceAuthorityLevels = map[string]int{
		ListSourceFederal:  4,
		ListSourceState:    3,
		ListSourceInternal: 2,
		ListSourceCustom:   1,
	}

	// Sources that require regulatory compliance
	regulatorySources = map[string]bool{
		ListSourceFederal: true,
		ListSourceState:   true,
	}

	// Sources that can be modified by users
	userModifiableSources = map[string]bool{
		ListSourceInternal: true,
		ListSourceCustom:   true,
	}
)

// NewListSource creates a new ListSource value object with validation
func NewListSource(source string) (ListSource, error) {
	if source == "" {
		return ListSource{}, errors.NewValidationError("EMPTY_LIST_SOURCE",
			"list source cannot be empty")
	}

	// Normalize source
	normalized := strings.ToLower(strings.TrimSpace(source))

	if !supportedSources[normalized] {
		return ListSource{}, errors.NewValidationError("UNSUPPORTED_LIST_SOURCE",
			fmt.Sprintf("list source '%s' is not supported", source))
	}

	return ListSource{source: normalized}, nil
}

// MustNewListSource creates ListSource and panics on error (for constants/tests)
func MustNewListSource(source string) ListSource {
	ls, err := NewListSource(source)
	if err != nil {
		panic(err)
	}
	return ls
}

// Standard list sources
func FederalListSource() ListSource {
	return MustNewListSource(ListSourceFederal)
}

func StateListSource() ListSource {
	return MustNewListSource(ListSourceState)
}

func InternalListSource() ListSource {
	return MustNewListSource(ListSourceInternal)
}

func CustomListSource() ListSource {
	return MustNewListSource(ListSourceCustom)
}

// String returns the source string
func (ls ListSource) String() string {
	return ls.source
}

// Value returns the underlying source value
func (ls ListSource) Value() string {
	return ls.source
}

// IsValid checks if the list source is valid
func (ls ListSource) IsValid() bool {
	return ls.source != "" && supportedSources[ls.source]
}

// IsEmpty checks if the source is empty
func (ls ListSource) IsEmpty() bool {
	return ls.source == ""
}

// Equal checks if two ListSource values are equal
func (ls ListSource) Equal(other ListSource) bool {
	return ls.source == other.source
}

// DisplayName returns the human-readable name for the source
func (ls ListSource) DisplayName() string {
	if name, ok := sourceDisplayNames[ls.source]; ok {
		return name
	}
	return strings.Title(ls.source) + " DNC List"
}

// AuthorityLevel returns the authority level of the source (higher = more authoritative)
func (ls ListSource) AuthorityLevel() int {
	if level, ok := sourceAuthorityLevels[ls.source]; ok {
		return level
	}
	return 0
}

// IsRegulatory checks if the source is a regulatory/government source
func (ls ListSource) IsRegulatory() bool {
	return regulatorySources[ls.source]
}

// IsUserModifiable checks if the source can be modified by users
func (ls ListSource) IsUserModifiable() bool {
	return userModifiableSources[ls.source]
}

// IsFederal checks if the source is federal
func (ls ListSource) IsFederal() bool {
	return ls.source == ListSourceFederal
}

// IsState checks if the source is state
func (ls ListSource) IsState() bool {
	return ls.source == ListSourceState
}

// IsInternal checks if the source is internal
func (ls ListSource) IsInternal() bool {
	return ls.source == ListSourceInternal
}

// IsCustom checks if the source is custom
func (ls ListSource) IsCustom() bool {
	return ls.source == ListSourceCustom
}

// HasHigherAuthority compares authority levels with another source
func (ls ListSource) HasHigherAuthority(other ListSource) bool {
	return ls.AuthorityLevel() > other.AuthorityLevel()
}

// RequiresCompliance checks if the source requires compliance tracking
func (ls ListSource) RequiresCompliance() bool {
	return ls.IsRegulatory()
}

// GetComplianceCode returns the compliance code for regulatory sources
func (ls ListSource) GetComplianceCode() string {
	switch ls.source {
	case ListSourceFederal:
		return "FEDERAL_DNC"
	case ListSourceState:
		return "STATE_DNC"
	default:
		return ""
	}
}

// ValidateForOperation validates if the source is appropriate for a specific operation
func (ls ListSource) ValidateForOperation(operation string) error {
	switch strings.ToLower(operation) {
	case "import":
		if ls.IsUserModifiable() {
			return nil
		}
		return errors.NewValidationError("INVALID_OPERATION",
			fmt.Sprintf("cannot import to %s list source", ls.DisplayName()))
	case "export":
		// All sources can be exported
		return nil
	case "modify", "delete":
		if !ls.IsUserModifiable() {
			return errors.NewValidationError("INVALID_OPERATION",
				fmt.Sprintf("cannot modify %s list source", ls.DisplayName()))
		}
		return nil
	case "query":
		// All sources can be queried
		return nil
	default:
		return errors.NewValidationError("UNKNOWN_OPERATION",
			fmt.Sprintf("unknown operation '%s'", operation))
	}
}

// GetRefreshPolicy returns the recommended refresh policy for the source
func (ls ListSource) GetRefreshPolicy() string {
	switch ls.source {
	case ListSourceFederal:
		return "monthly" // Federal DNC updates monthly
	case ListSourceState:
		return "weekly" // State lists may update more frequently
	case ListSourceInternal, ListSourceCustom:
		return "on-demand" // User-managed lists update on demand
	default:
		return "unknown"
	}
}

// GetPriority returns the priority for conflict resolution (higher = higher priority)
func (ls ListSource) GetPriority() int {
	// Same as authority level for now, but could be different
	return ls.AuthorityLevel()
}

// MarshalJSON implements JSON marshaling
func (ls ListSource) MarshalJSON() ([]byte, error) {
	return json.Marshal(ls.source)
}

// UnmarshalJSON implements JSON unmarshaling
func (ls *ListSource) UnmarshalJSON(data []byte) error {
	var source string
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}

	listSource, err := NewListSource(source)
	if err != nil {
		return err
	}

	*ls = listSource
	return nil
}

// Value implements driver.Valuer for database storage
func (ls ListSource) Value() (driver.Value, error) {
	if ls.source == "" {
		return nil, nil
	}
	return ls.source, nil
}

// Scan implements sql.Scanner for database retrieval
func (ls *ListSource) Scan(value interface{}) error {
	if value == nil {
		*ls = ListSource{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into ListSource", value)
	}

	if str == "" {
		*ls = ListSource{}
		return nil
	}

	listSource, err := NewListSource(str)
	if err != nil {
		return err
	}

	*ls = listSource
	return nil
}

// GetSupportedSources returns all supported list sources
func GetSupportedSources() []string {
	sources := make([]string, 0, len(supportedSources))
	for source := range supportedSources {
		sources = append(sources, source)
	}
	return sources
}

// GetRegulatorySourceNames returns all regulatory source names
func GetRegulatorySourceNames() []string {
	sources := make([]string, 0, len(regulatorySources))
	for source := range regulatorySources {
		if regulatorySources[source] {
			sources = append(sources, sourceDisplayNames[source])
		}
	}
	return sources
}

// ValidateListSource validates that a string could be a valid list source
func ValidateListSource(source string) error {
	if source == "" {
		return errors.NewValidationError("EMPTY_LIST_SOURCE", "list source cannot be empty")
	}

	normalized := strings.ToLower(strings.TrimSpace(source))
	if !supportedSources[normalized] {
		return errors.NewValidationError("UNSUPPORTED_LIST_SOURCE",
			fmt.Sprintf("list source '%s' is not supported", source))
	}

	return nil
}