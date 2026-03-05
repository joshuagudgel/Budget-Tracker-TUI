package storage

import (
	"strings"
	"time"
)

// getDateFormatsToTry returns common date format variations to try
// Includes the primary format plus alternative separators (slash/dash variants)
func (cts *CSVTemplateStore) getDateFormatsToTry(primaryFormat string) []string {
	formats := []string{primaryFormat} // Always try the template's format first

	// Add alternative separators (dash and slash versions of the same format)
	if strings.Contains(primaryFormat, "/") {
		// If format uses slashes, also try dashes
		dashFormat := strings.ReplaceAll(primaryFormat, "/", "-")
		if dashFormat != primaryFormat {
			formats = append(formats, dashFormat)
		}
	} else if strings.Contains(primaryFormat, "-") {
		// If format uses dashes, also try slashes
		slashFormat := strings.ReplaceAll(primaryFormat, "-", "/")
		if slashFormat != primaryFormat {
			formats = append(formats, slashFormat)
		}
	}

	return formats
}

// tryParseDateWithMultipleFormats attempts to parse a date string with multiple common formats
// Returns true if date parses successfully with any format, false otherwise
func (cts *CSVTemplateStore) tryParseDateWithMultipleFormats(dateStr string, formats []string) bool {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return false // Empty date is invalid
	}

	for _, format := range formats {
		if _, err := time.Parse(format, trimmed); err == nil {
			return true // Success with this format
		}
	}
	return false
}
