package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ExportFormat represents a supported export format for audit data
type ExportFormat struct {
	format string
}

// Supported export formats
const (
	FormatJSON     = "json"
	FormatCSV      = "csv"
	FormatXML      = "xml"
	FormatParquet  = "parquet"
	FormatAvro     = "avro"
	FormatPDF      = "pdf"
	FormatExcel    = "xlsx"
	FormatPlainText = "txt"
)

var (
	// Map of format to MIME types
	formatMimeTypes = map[string]string{
		FormatJSON:      "application/json",
		FormatCSV:       "text/csv",
		FormatXML:       "application/xml",
		FormatParquet:   "application/parquet",
		FormatAvro:      "application/avro",
		FormatPDF:       "application/pdf",
		FormatExcel:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		FormatPlainText: "text/plain",
	}

	// Map of format to file extensions
	formatExtensions = map[string]string{
		FormatJSON:      ".json",
		FormatCSV:       ".csv",
		FormatXML:       ".xml",
		FormatParquet:   ".parquet",
		FormatAvro:      ".avro",
		FormatPDF:       ".pdf",
		FormatExcel:     ".xlsx",
		FormatPlainText: ".txt",
	}

	// Supported formats for validation
	supportedFormats = map[string]bool{
		FormatJSON:      true,
		FormatCSV:       true,
		FormatXML:       true,
		FormatParquet:   true,
		FormatAvro:      true,
		FormatPDF:       true,
		FormatExcel:     true,
		FormatPlainText: true,
	}

	// Human-readable format names
	formatNames = map[string]string{
		FormatJSON:      "JSON",
		FormatCSV:       "CSV",
		FormatXML:       "XML",
		FormatParquet:   "Parquet",
		FormatAvro:      "Apache Avro",
		FormatPDF:       "PDF",
		FormatExcel:     "Excel",
		FormatPlainText: "Plain Text",
	}

	// Formats suitable for structured data
	structuredFormats = map[string]bool{
		FormatJSON:    true,
		FormatCSV:     true,
		FormatXML:     true,
		FormatParquet: true,
		FormatAvro:    true,
		FormatExcel:   true,
	}

	// Formats suitable for archival/compliance
	archivalFormats = map[string]bool{
		FormatJSON:    true,
		FormatXML:     true,
		FormatParquet: true,
		FormatPDF:     true,
	}

	// Formats that support compression
	compressibleFormats = map[string]bool{
		FormatJSON:      true,
		FormatCSV:       true,
		FormatXML:       true,
		FormatPlainText: true,
	}
)

// NewExportFormat creates a new ExportFormat value object with validation
func NewExportFormat(format string) (ExportFormat, error) {
	if format == "" {
		return ExportFormat{}, errors.NewValidationError("EMPTY_FORMAT", 
			"export format cannot be empty")
	}

	// Normalize format
	normalized := strings.ToLower(strings.TrimSpace(format))

	// Remove leading dot if present
	if strings.HasPrefix(normalized, ".") {
		normalized = normalized[1:]
	}

	if !supportedFormats[normalized] {
		return ExportFormat{}, errors.NewValidationError("UNSUPPORTED_FORMAT", 
			fmt.Sprintf("export format '%s' is not supported", format))
	}

	return ExportFormat{format: normalized}, nil
}

// NewExportFormatFromMimeType creates ExportFormat from MIME type
func NewExportFormatFromMimeType(mimeType string) (ExportFormat, error) {
	if mimeType == "" {
		return ExportFormat{}, errors.NewValidationError("EMPTY_MIME_TYPE", 
			"MIME type cannot be empty")
	}

	// Normalize MIME type
	normalized := strings.ToLower(strings.TrimSpace(mimeType))

	// Find format by MIME type
	for format, mt := range formatMimeTypes {
		if mt == normalized {
			return ExportFormat{format: format}, nil
		}
	}

	return ExportFormat{}, errors.NewValidationError("UNSUPPORTED_MIME_TYPE", 
		fmt.Sprintf("MIME type '%s' is not supported", mimeType))
}

// NewExportFormatFromExtension creates ExportFormat from file extension
func NewExportFormatFromExtension(extension string) (ExportFormat, error) {
	if extension == "" {
		return ExportFormat{}, errors.NewValidationError("EMPTY_EXTENSION", 
			"file extension cannot be empty")
	}

	// Normalize extension
	normalized := strings.ToLower(strings.TrimSpace(extension))
	if !strings.HasPrefix(normalized, ".") {
		normalized = "." + normalized
	}

	// Find format by extension
	for format, ext := range formatExtensions {
		if ext == normalized {
			return ExportFormat{format: format}, nil
		}
	}

	return ExportFormat{}, errors.NewValidationError("UNSUPPORTED_EXTENSION", 
		fmt.Sprintf("file extension '%s' is not supported", extension))
}

// NewExportFormatFromFilename creates ExportFormat from filename
func NewExportFormatFromFilename(filename string) (ExportFormat, error) {
	if filename == "" {
		return ExportFormat{}, errors.NewValidationError("EMPTY_FILENAME", 
			"filename cannot be empty")
	}

	extension := filepath.Ext(filename)
	if extension == "" {
		return ExportFormat{}, errors.NewValidationError("NO_EXTENSION", 
			"filename must have an extension")
	}

	return NewExportFormatFromExtension(extension)
}

// MustNewExportFormat creates ExportFormat and panics on error (for constants/tests)
func MustNewExportFormat(format string) ExportFormat {
	ef, err := NewExportFormat(format)
	if err != nil {
		panic(err)
	}
	return ef
}

// Standard export formats
func JSONFormat() ExportFormat {
	return MustNewExportFormat(FormatJSON)
}

func CSVFormat() ExportFormat {
	return MustNewExportFormat(FormatCSV)
}

func XMLFormat() ExportFormat {
	return MustNewExportFormat(FormatXML)
}

func ParquetFormat() ExportFormat {
	return MustNewExportFormat(FormatParquet)
}

func PDFFormat() ExportFormat {
	return MustNewExportFormat(FormatPDF)
}

// String returns the format string
func (ef ExportFormat) String() string {
	return ef.format
}

// Format returns the format string (alias for String)
func (ef ExportFormat) Format() string {
	return ef.format
}

// IsEmpty checks if the format is empty
func (ef ExportFormat) IsEmpty() bool {
	return ef.format == ""
}

// Equal checks if two ExportFormat values are equal
func (ef ExportFormat) Equal(other ExportFormat) bool {
	return ef.format == other.format
}

// MimeType returns the MIME type for the format
func (ef ExportFormat) MimeType() string {
	if mimeType, ok := formatMimeTypes[ef.format]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

// Extension returns the file extension for the format
func (ef ExportFormat) Extension() string {
	if extension, ok := formatExtensions[ef.format]; ok {
		return extension
	}
	return ".bin"
}

// Name returns the human-readable name for the format
func (ef ExportFormat) Name() string {
	if name, ok := formatNames[ef.format]; ok {
		return name
	}
	return strings.ToUpper(ef.format)
}

// IsStructured checks if the format supports structured data
func (ef ExportFormat) IsStructured() bool {
	return structuredFormats[ef.format]
}

// IsArchival checks if the format is suitable for archival/compliance
func (ef ExportFormat) IsArchival() bool {
	return archivalFormats[ef.format]
}

// IsCompressible checks if the format can be compressed
func (ef ExportFormat) IsCompressible() bool {
	return compressibleFormats[ef.format]
}

// IsBinary checks if the format produces binary output
func (ef ExportFormat) IsBinary() bool {
	return ef.format == FormatParquet || ef.format == FormatAvro || 
		   ef.format == FormatPDF || ef.format == FormatExcel
}

// IsTextBased checks if the format produces text output
func (ef ExportFormat) IsTextBased() bool {
	return !ef.IsBinary()
}

// SupportsCompression returns whether the format benefits from compression
func (ef ExportFormat) SupportsCompression() bool {
	return ef.IsCompressible() && ef.IsTextBased()
}

// GetContentDisposition returns appropriate Content-Disposition header value
func (ef ExportFormat) GetContentDisposition(filename string) string {
	if filename == "" {
		filename = "export" + ef.Extension()
	}
	
	// Ensure filename has correct extension
	if filepath.Ext(filename) != ef.Extension() {
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ef.Extension()
	}

	return fmt.Sprintf("attachment; filename=%s", filename)
}

// ValidateForUseCase checks if the format is appropriate for a specific use case
func (ef ExportFormat) ValidateForUseCase(useCase string) error {
	switch strings.ToLower(useCase) {
	case "audit", "compliance":
		if !ef.IsArchival() {
			return errors.NewValidationError("INAPPROPRIATE_FORMAT", 
				fmt.Sprintf("format '%s' is not suitable for audit/compliance", ef.format))
		}
	case "analytics", "reporting":
		if !ef.IsStructured() {
			return errors.NewValidationError("INAPPROPRIATE_FORMAT", 
				fmt.Sprintf("format '%s' is not suitable for analytics/reporting", ef.format))
		}
	case "backup", "archive":
		if !ef.IsArchival() && !ef.IsStructured() {
			return errors.NewValidationError("INAPPROPRIATE_FORMAT", 
				fmt.Sprintf("format '%s' is not suitable for backup/archive", ef.format))
		}
	}
	return nil
}

// GetCompressionRecommendation returns recommended compression for the format
func (ef ExportFormat) GetCompressionRecommendation() string {
	if !ef.SupportsCompression() {
		return "none"
	}

	switch ef.format {
	case FormatJSON, FormatXML:
		return "gzip"
	case FormatCSV, FormatPlainText:
		return "gzip"
	default:
		return "none"
	}
}

// FormatDisplay returns a formatted string for display
func (ef ExportFormat) FormatDisplay() string {
	if ef.IsEmpty() {
		return "<invalid>"
	}
	return fmt.Sprintf("%s (%s)", ef.Name(), ef.Extension())
}

// FormatWithMime returns format with MIME type for debugging
func (ef ExportFormat) FormatWithMime() string {
	if ef.IsEmpty() {
		return "<invalid>"
	}
	return fmt.Sprintf("%s [%s]", ef.Name(), ef.MimeType())
}

// MarshalJSON implements JSON marshaling
func (ef ExportFormat) MarshalJSON() ([]byte, error) {
	return json.Marshal(ef.format)
}

// UnmarshalJSON implements JSON unmarshaling
func (ef *ExportFormat) UnmarshalJSON(data []byte) error {
	var format string
	if err := json.Unmarshal(data, &format); err != nil {
		return err
	}

	exportFormat, err := NewExportFormat(format)
	if err != nil {
		return err
	}

	*ef = exportFormat
	return nil
}

// Value implements driver.Valuer for database storage
func (ef ExportFormat) Value() (driver.Value, error) {
	if ef.format == "" {
		return nil, nil
	}
	return ef.format, nil
}

// Scan implements sql.Scanner for database retrieval
func (ef *ExportFormat) Scan(value interface{}) error {
	if value == nil {
		*ef = ExportFormat{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into ExportFormat", value)
	}

	if str == "" {
		*ef = ExportFormat{}
		return nil
	}

	exportFormat, err := NewExportFormat(str)
	if err != nil {
		return err
	}

	*ef = exportFormat
	return nil
}

// ExportFormatSet represents a set of export formats
type ExportFormatSet struct {
	formats map[string]ExportFormat
}

// NewExportFormatSet creates a new set of export formats
func NewExportFormatSet(formats ...ExportFormat) *ExportFormatSet {
	set := &ExportFormatSet{
		formats: make(map[string]ExportFormat),
	}
	
	for _, format := range formats {
		if !format.IsEmpty() {
			set.formats[format.String()] = format
		}
	}
	
	return set
}

// Add adds a format to the set
func (efs *ExportFormatSet) Add(format ExportFormat) {
	if !format.IsEmpty() {
		efs.formats[format.String()] = format
	}
}

// Contains checks if the set contains a format
func (efs *ExportFormatSet) Contains(format ExportFormat) bool {
	_, exists := efs.formats[format.String()]
	return exists
}

// Remove removes a format from the set
func (efs *ExportFormatSet) Remove(format ExportFormat) {
	delete(efs.formats, format.String())
}

// ToSlice returns all formats as a slice
func (efs *ExportFormatSet) ToSlice() []ExportFormat {
	result := make([]ExportFormat, 0, len(efs.formats))
	for _, format := range efs.formats {
		result = append(result, format)
	}
	return result
}

// Size returns the number of formats in the set
func (efs *ExportFormatSet) Size() int {
	return len(efs.formats)
}

// IsEmpty checks if the set is empty
func (efs *ExportFormatSet) IsEmpty() bool {
	return len(efs.formats) == 0
}

// ValidationError represents validation errors for export formats
type ExportFormatValidationError struct {
	Format string
	Reason string
}

func (e ExportFormatValidationError) Error() string {
	return fmt.Sprintf("invalid export format '%s': %s", e.Format, e.Reason)
}

// GetSupportedFormats returns all supported export formats
func GetSupportedFormats() []string {
	formats := make([]string, 0, len(supportedFormats))
	for format := range supportedFormats {
		formats = append(formats, format)
	}
	return formats
}

// GetSupportedFormatNames returns all supported format names
func GetSupportedFormatNames() []string {
	names := make([]string, 0, len(formatNames))
	for _, name := range formatNames {
		names = append(names, name)
	}
	return names
}

// ValidateExportFormat validates that a string could be a valid export format
func ValidateExportFormat(format string) error {
	if format == "" {
		return errors.NewValidationError("EMPTY_FORMAT", "export format cannot be empty")
	}

	normalized := strings.ToLower(strings.TrimSpace(format))
	if strings.HasPrefix(normalized, ".") {
		normalized = normalized[1:]
	}

	if !supportedFormats[normalized] {
		return errors.NewValidationError("UNSUPPORTED_FORMAT", 
			fmt.Sprintf("export format '%s' is not supported", format))
	}

	return nil
}

// GuessFormatFromContent attempts to guess format from content type or file signature
func GuessFormatFromContent(contentType string, data []byte) (ExportFormat, error) {
	// Try MIME type first
	if contentType != "" {
		if format, err := NewExportFormatFromMimeType(contentType); err == nil {
			return format, nil
		}
	}

	// Try to detect from file signature/magic bytes
	if len(data) >= 4 {
		// Check for common file signatures
		if data[0] == '{' || (data[0] == '[' && data[1] == '{') {
			return JSONFormat(), nil
		}
		if data[0] == '<' && data[1] == '?' {
			return XMLFormat(), nil
		}
		if string(data[:4]) == "PAR1" {
			return ParquetFormat(), nil
		}
		if string(data[:4]) == "%PDF" {
			return PDFFormat(), nil
		}
	}

	return ExportFormat{}, errors.NewValidationError("UNKNOWN_FORMAT", 
		"unable to determine export format from content")
}