package types

import (
	"fmt"
	"strings"
	"time"
)

// TryParseMultipleDateFormats attempts to parse a date string with common formats
// Returns the parsed time.Time or error if no format matches
// Tries ISO 8601 and common user-friendly formats
func TryParseMultipleDateFormats(dateStr string) (time.Time, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("date cannot be empty")
	}

	// Try common date formats in order of likelihood
	formats := []string{
		"2006-01-02", // ISO 8601 (YYYY-MM-DD)
		"01/02/2006", // MM/DD/YYYY
		"01-02-2006", // MM-DD-YYYY
		"2006/01/02", // YYYY/MM/DD
		"02/01/2006", // DD/MM/YYYY
		"02-01-2006", // DD-MM-YYYY
		"1/2/2006",   // M/D/YYYY
		"2/1/2006",   // D/M/YYYY
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, trimmed); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse date '%s' with any recognized format", trimmed)
}

// NormalizeDateToISO8601 normalizes a date string to ISO 8601 format (YYYY-MM-DD)
// Accepts input in template format hint or common formats
// Returns YYYY-MM-DD format suitable for ISO DATE storage in SQLite
func NormalizeDateToISO8601(dateStr string, templateFormat string) (string, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return "", fmt.Errorf("date cannot be empty")
	}

	var parsedTime time.Time
	var err error

	// First try the template format if provided
	if templateFormat != "" {
		parsedTime, err = time.Parse(templateFormat, trimmed)
		if err == nil {
			return parsedTime.Format("2006-01-02"), nil
		}
	}

	// Fall back to trying multiple common formats
	parsedTime, err = TryParseMultipleDateFormats(trimmed)
	if err != nil {
		return "", err
	}

	return parsedTime.Format("2006-01-02"), nil
}
