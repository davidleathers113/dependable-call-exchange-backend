package values

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExportFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid json format",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "valid csv format",
			format:  "csv",
			wantErr: false,
		},
		{
			name:    "valid uppercase format",
			format:  "JSON",
			wantErr: false,
		},
		{
			name:    "format with leading dot",
			format:  ".json",
			wantErr: false,
		},
		{
			name:    "format with whitespace",
			format:  " json ",
			wantErr: false,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
			errCode: "EMPTY_FORMAT",
		},
		{
			name:    "unsupported format",
			format:  "unsupported",
			wantErr: true,
			errCode: "UNSUPPORTED_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := NewExportFormat(tt.format)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
				assert.True(t, ef.IsEmpty())
			} else {
				assert.NoError(t, err)
				assert.False(t, ef.IsEmpty())
				expectedFormat := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tt.format, ".")))
				assert.Equal(t, expectedFormat, ef.String())
			}
		})
	}
}

func TestNewExportFormatFromMimeType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected string
		wantErr  bool
		errCode  string
	}{
		{
			name:     "json mime type",
			mimeType: "application/json",
			expected: FormatJSON,
			wantErr:  false,
		},
		{
			name:     "csv mime type",
			mimeType: "text/csv",
			expected: FormatCSV,
			wantErr:  false,
		},
		{
			name:     "pdf mime type",
			mimeType: "application/pdf",
			expected: FormatPDF,
			wantErr:  false,
		},
		{
			name:     "excel mime type",
			mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			expected: FormatExcel,
			wantErr:  false,
		},
		{
			name:     "empty mime type",
			mimeType: "",
			wantErr:  true,
			errCode:  "EMPTY_MIME_TYPE",
		},
		{
			name:     "unsupported mime type",
			mimeType: "application/octet-stream",
			wantErr:  true,
			errCode:  "UNSUPPORTED_MIME_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := NewExportFormatFromMimeType(tt.mimeType)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ef.String())
			}
		})
	}
}

func TestNewExportFormatFromExtension(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		expected  string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "json extension with dot",
			extension: ".json",
			expected:  FormatJSON,
			wantErr:   false,
		},
		{
			name:      "json extension without dot",
			extension: "json",
			expected:  FormatJSON,
			wantErr:   false,
		},
		{
			name:      "csv extension",
			extension: ".csv",
			expected:  FormatCSV,
			wantErr:   false,
		},
		{
			name:      "xlsx extension",
			extension: ".xlsx",
			expected:  FormatExcel,
			wantErr:   false,
		},
		{
			name:      "empty extension",
			extension: "",
			wantErr:   true,
			errCode:   "EMPTY_EXTENSION",
		},
		{
			name:      "unsupported extension",
			extension: ".unknown",
			wantErr:   true,
			errCode:   "UNSUPPORTED_EXTENSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := NewExportFormatFromExtension(tt.extension)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ef.String())
			}
		})
	}
}

func TestNewExportFormatFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
		wantErr  bool
		errCode  string
	}{
		{
			name:     "json filename",
			filename: "data.json",
			expected: FormatJSON,
			wantErr:  false,
		},
		{
			name:     "csv filename with path",
			filename: "/path/to/data.csv",
			expected: FormatCSV,
			wantErr:  false,
		},
		{
			name:     "empty filename",
			filename: "",
			wantErr:  true,
			errCode:  "EMPTY_FILENAME",
		},
		{
			name:     "filename without extension",
			filename: "data",
			wantErr:  true,
			errCode:  "NO_EXTENSION",
		},
		{
			name:     "filename with unsupported extension",
			filename: "data.unknown",
			wantErr:  true,
			errCode:  "UNSUPPORTED_EXTENSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := NewExportFormatFromFilename(tt.filename)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ef.String())
			}
		})
	}
}

func TestStandardExportFormats(t *testing.T) {
	tests := []struct {
		factory  func() ExportFormat
		expected string
	}{
		{JSONFormat, FormatJSON},
		{CSVFormat, FormatCSV},
		{XMLFormat, FormatXML},
		{ParquetFormat, FormatParquet},
		{PDFFormat, FormatPDF},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			ef := tt.factory()
			assert.Equal(t, tt.expected, ef.String())
			assert.False(t, ef.IsEmpty())
		})
	}
}

func TestExportFormat_Equal(t *testing.T) {
	ef1 := JSONFormat()
	ef2 := JSONFormat()
	ef3 := CSVFormat()

	assert.True(t, ef1.Equal(ef2))
	assert.False(t, ef1.Equal(ef3))
	assert.True(t, ef1.Equal(ef1))
}

func TestExportFormat_MimeType(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{JSONFormat(), "application/json"},
		{CSVFormat(), "text/csv"},
		{XMLFormat(), "application/xml"},
		{PDFFormat(), "application/pdf"},
		{ParquetFormat(), "application/parquet"},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.format.MimeType())
		})
	}
}

func TestExportFormat_Extension(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{JSONFormat(), ".json"},
		{CSVFormat(), ".csv"},
		{XMLFormat(), ".xml"},
		{PDFFormat(), ".pdf"},
		{MustNewExportFormat(FormatExcel), ".xlsx"},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.format.Extension())
		})
	}
}

func TestExportFormat_Name(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{JSONFormat(), "JSON"},
		{CSVFormat(), "CSV"},
		{XMLFormat(), "XML"},
		{PDFFormat(), "PDF"},
		{MustNewExportFormat(FormatExcel), "Excel"},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.format.Name())
		})
	}
}

func TestExportFormat_Properties(t *testing.T) {
	tests := []struct {
		format         ExportFormat
		isStructured   bool
		isArchival     bool
		isCompressible bool
		isBinary       bool
		isTextBased    bool
	}{
		{JSONFormat(), true, true, true, false, true},
		{CSVFormat(), true, false, true, false, true},
		{XMLFormat(), true, true, true, false, true},
		{PDFFormat(), false, true, false, true, false},
		{ParquetFormat(), true, true, false, true, false},
		{MustNewExportFormat(FormatExcel), true, false, false, true, false},
		{MustNewExportFormat(FormatPlainText), false, false, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			assert.Equal(t, tt.isStructured, tt.format.IsStructured(), "IsStructured")
			assert.Equal(t, tt.isArchival, tt.format.IsArchival(), "IsArchival")
			assert.Equal(t, tt.isCompressible, tt.format.IsCompressible(), "IsCompressible")
			assert.Equal(t, tt.isBinary, tt.format.IsBinary(), "IsBinary")
			assert.Equal(t, tt.isTextBased, tt.format.IsTextBased(), "IsTextBased")
		})
	}
}

func TestExportFormat_SupportsCompression(t *testing.T) {
	// Text-based and compressible formats should support compression
	jsonFormat := JSONFormat()
	assert.True(t, jsonFormat.SupportsCompression())

	csvFormat := CSVFormat()
	assert.True(t, csvFormat.SupportsCompression())

	// Binary formats should not support compression
	pdfFormat := PDFFormat()
	assert.False(t, pdfFormat.SupportsCompression())

	parquetFormat := ParquetFormat()
	assert.False(t, parquetFormat.SupportsCompression())
}

func TestExportFormat_GetContentDisposition(t *testing.T) {
	ef := JSONFormat()

	// Test with provided filename
	disposition := ef.GetContentDisposition("data.json")
	assert.Equal(t, "attachment; filename=data.json", disposition)

	// Test with filename that needs extension correction
	disposition = ef.GetContentDisposition("data.csv")
	assert.Equal(t, "attachment; filename=data.json", disposition)

	// Test with empty filename
	disposition = ef.GetContentDisposition("")
	assert.Equal(t, "attachment; filename=export.json", disposition)
}

func TestExportFormat_ValidateForUseCase(t *testing.T) {
	tests := []struct {
		format  ExportFormat
		useCase string
		wantErr bool
	}{
		// Audit/compliance use case
		{JSONFormat(), "audit", false},    // Archival format
		{XMLFormat(), "compliance", false}, // Archival format
		{CSVFormat(), "audit", true},      // Not archival
		
		// Analytics/reporting use case
		{JSONFormat(), "analytics", false},  // Structured format
		{CSVFormat(), "reporting", false},   // Structured format
		{PDFFormat(), "analytics", true},    // Not structured
		
		// Backup/archive use case
		{JSONFormat(), "backup", false},    // Archival and structured
		{ParquetFormat(), "archive", false}, // Archival and structured
		{MustNewExportFormat(FormatPlainText), "backup", true}, // Neither archival nor structured
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.format.String(), tt.useCase), func(t *testing.T) {
			err := tt.format.ValidateForUseCase(tt.useCase)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "INAPPROPRIATE_FORMAT")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportFormat_GetCompressionRecommendation(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{JSONFormat(), "gzip"},
		{CSVFormat(), "gzip"},
		{XMLFormat(), "gzip"},
		{MustNewExportFormat(FormatPlainText), "gzip"},
		{PDFFormat(), "none"},
		{ParquetFormat(), "none"},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.format.GetCompressionRecommendation())
		})
	}
}

func TestExportFormat_FormatDisplay(t *testing.T) {
	ef := JSONFormat()
	emptyEf := ExportFormat{}

	display := ef.FormatDisplay()
	assert.Equal(t, "JSON (.json)", display)

	emptyDisplay := emptyEf.FormatDisplay()
	assert.Equal(t, "<invalid>", emptyDisplay)
}

func TestExportFormat_FormatWithMime(t *testing.T) {
	ef := JSONFormat()
	emptyEf := ExportFormat{}

	formatted := ef.FormatWithMime()
	assert.Equal(t, "JSON [application/json]", formatted)

	emptyFormatted := emptyEf.FormatWithMime()
	assert.Equal(t, "<invalid>", emptyFormatted)
}

func TestExportFormat_JSON(t *testing.T) {
	ef := JSONFormat()

	// Test marshaling
	data, err := json.Marshal(ef)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled ExportFormat
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, ef.Equal(unmarshaled))
}

func TestExportFormat_Database(t *testing.T) {
	ef := JSONFormat()

	// Test Value
	value, err := ef.Value()
	require.NoError(t, err)
	assert.Equal(t, FormatJSON, value)

	// Test Scan
	var scanned ExportFormat
	err = scanned.Scan(value)
	require.NoError(t, err)
	assert.True(t, ef.Equal(scanned))

	// Test Scan with nil
	var nilEf ExportFormat
	err = nilEf.Scan(nil)
	require.NoError(t, err)
	assert.True(t, nilEf.IsEmpty())

	// Test Scan with bytes
	var bytesEf ExportFormat
	err = bytesEf.Scan([]byte(FormatJSON))
	require.NoError(t, err)
	assert.True(t, ef.Equal(bytesEf))
}

func TestExportFormatSet(t *testing.T) {
	ef1 := JSONFormat()
	ef2 := CSVFormat()
	ef3 := XMLFormat()

	// Test NewExportFormatSet
	set := NewExportFormatSet(ef1, ef2, ef3)
	assert.Equal(t, 3, set.Size())
	assert.False(t, set.IsEmpty())

	// Test Contains
	assert.True(t, set.Contains(ef1))
	assert.True(t, set.Contains(ef2))
	assert.True(t, set.Contains(ef3))
	assert.False(t, set.Contains(PDFFormat()))

	// Test Add
	pdfFormat := PDFFormat()
	set.Add(pdfFormat)
	assert.Equal(t, 4, set.Size())
	assert.True(t, set.Contains(pdfFormat))

	// Test Remove
	set.Remove(ef1)
	assert.Equal(t, 3, set.Size())
	assert.False(t, set.Contains(ef1))

	// Test ToSlice
	slice := set.ToSlice()
	assert.Len(t, slice, 3)

	// Test with empty format (should be ignored)
	emptySet := NewExportFormatSet(ExportFormat{})
	assert.True(t, emptySet.IsEmpty())
}

func TestGetSupportedFormats(t *testing.T) {
	formats := GetSupportedFormats()
	assert.Contains(t, formats, FormatJSON)
	assert.Contains(t, formats, FormatCSV)
	assert.Contains(t, formats, FormatXML)
	assert.Contains(t, formats, FormatPDF)
	assert.Len(t, formats, len(supportedFormats))
}

func TestGetSupportedFormatNames(t *testing.T) {
	names := GetSupportedFormatNames()
	assert.Contains(t, names, "JSON")
	assert.Contains(t, names, "CSV")
	assert.Contains(t, names, "XML")
	assert.Contains(t, names, "PDF")
	assert.Len(t, names, len(formatNames))
}

func TestValidateExportFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "valid format",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "valid format with dot",
			format:  ".json",
			wantErr: false,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
		{
			name:    "unsupported format",
			format:  "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExportFormat(tt.format)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGuessFormatFromContent(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        []byte
		expected    string
		wantErr     bool
	}{
		{
			name:        "json from mime type",
			contentType: "application/json",
			data:        []byte{},
			expected:    FormatJSON,
			wantErr:     false,
		},
		{
			name:        "json from content",
			contentType: "",
			data:        []byte(`{"key": "value"}`),
			expected:    FormatJSON,
			wantErr:     false,
		},
		{
			name:        "json array from content",
			contentType: "",
			data:        []byte(`[{"key": "value"}]`),
			expected:    FormatJSON,
			wantErr:     false,
		},
		{
			name:        "xml from content",
			contentType: "",
			data:        []byte(`<?xml version="1.0"?>`),
			expected:    FormatXML,
			wantErr:     false,
		},
		{
			name:        "pdf from content",
			contentType: "",
			data:        []byte(`%PDF-1.4`),
			expected:    FormatPDF,
			wantErr:     false,
		},
		{
			name:        "parquet from content",
			contentType: "",
			data:        []byte(`PAR1`),
			expected:    FormatParquet,
			wantErr:     false,
		},
		{
			name:        "unknown format",
			contentType: "",
			data:        []byte(`unknown content`),
			expected:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := GuessFormatFromContent(tt.contentType, tt.data)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "UNKNOWN_FORMAT")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ef.String())
			}
		})
	}
}

// Property-based tests
func TestExportFormat_PropertyTests(t *testing.T) {
	// Property: JSON marshaling/unmarshaling should preserve equality
	t.Run("json_roundtrip_preserves_equality", func(t *testing.T) {
		original := JSONFormat()

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored ExportFormat
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: Database value/scan roundtrip should preserve equality
	t.Run("database_roundtrip_preserves_equality", func(t *testing.T) {
		original := CSVFormat()

		value, err := original.Value()
		require.NoError(t, err)

		var restored ExportFormat
		err = restored.Scan(value)
		require.NoError(t, err)

		assert.True(t, original.Equal(restored))
	})

	// Property: MimeType should always return a valid MIME type
	t.Run("mime_type_always_valid", func(t *testing.T) {
		formats := []ExportFormat{
			JSONFormat(),
			CSVFormat(),
			XMLFormat(),
			PDFFormat(),
			ParquetFormat(),
		}

		for _, format := range formats {
			mimeType := format.MimeType()
			assert.NotEmpty(t, mimeType)
			assert.Contains(t, mimeType, "/") // Valid MIME type should contain "/"
		}
	})

	// Property: Extension should always start with dot
	t.Run("extension_always_starts_with_dot", func(t *testing.T) {
		formats := []ExportFormat{
			JSONFormat(),
			CSVFormat(),
			XMLFormat(),
			PDFFormat(),
			ParquetFormat(),
		}

		for _, format := range formats {
			extension := format.Extension()
			assert.NotEmpty(t, extension)
			assert.True(t, strings.HasPrefix(extension, "."), "Extension should start with dot")
		}
	})

	// Property: Binary and text-based should be mutually exclusive
	t.Run("binary_and_text_mutually_exclusive", func(t *testing.T) {
		formats := []ExportFormat{
			JSONFormat(),
			CSVFormat(),
			XMLFormat(),
			PDFFormat(),
			ParquetFormat(),
		}

		for _, format := range formats {
			isBinary := format.IsBinary()
			isTextBased := format.IsTextBased()
			assert.NotEqual(t, isBinary, isTextBased, 
				"Format %s: binary and text-based should be mutually exclusive", format.String())
		}
	})
}

// Edge case tests
func TestExportFormat_EdgeCases(t *testing.T) {
	// Test invalid MIME type handling
	t.Run("invalid_mime_type", func(t *testing.T) {
		_, err := NewExportFormatFromMimeType("invalid-mime-type")
		assert.Error(t, err)
	})

	// Test case sensitivity
	t.Run("case_insensitive_creation", func(t *testing.T) {
		lowerCase, err := NewExportFormat("json")
		require.NoError(t, err)

		upperCase, err := NewExportFormat("JSON")
		require.NoError(t, err)

		mixedCase, err := NewExportFormat("Json")
		require.NoError(t, err)

		assert.True(t, lowerCase.Equal(upperCase))
		assert.True(t, lowerCase.Equal(mixedCase))
		assert.True(t, upperCase.Equal(mixedCase))
	})

	// Test whitespace handling
	t.Run("whitespace_handling", func(t *testing.T) {
		withWhitespace, err := NewExportFormat("  json  ")
		require.NoError(t, err)

		normal, err := NewExportFormat("json")
		require.NoError(t, err)

		assert.True(t, withWhitespace.Equal(normal))
	})
}

// Benchmark tests
func BenchmarkNewExportFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewExportFormat("json")
	}
}

func BenchmarkExportFormat_MimeType(b *testing.B) {
	ef := JSONFormat()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ef.MimeType()
	}
}

func BenchmarkExportFormat_ValidateForUseCase(b *testing.B) {
	ef := JSONFormat()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ef.ValidateForUseCase("audit")
	}
}