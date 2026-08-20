package dm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var damengDecimalTypePattern = regexp.MustCompile(`(?i)^(decimal|numeric)\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)$`)
var damengBooleanDefaultPattern = regexp.MustCompile(`(?i)\bdefault\s+(true|false)\b`)

// normalizeDamengDataType converts MySQL-specific GORM v1 type tags into
// types accepted by Dameng before AutoMigrate emits CREATE TABLE statements.
func normalizeDamengDataType(sqlType string) string {
	trimmedType := strings.TrimSpace(sqlType)
	switch strings.ToLower(trimmedType) {
	case "tinytext", "text", "mediumtext", "longtext":
		return "CLOB"
	case "tinyblob", "blob", "mediumblob", "longblob":
		return "BLOB"
	case "mediumint":
		return "INT"
	}

	return normalizeDamengDecimalPrecision(trimmedType)
}

// normalizeDamengAdditionalType translates GORM's Go boolean literals to the
// numeric defaults accepted by a Dameng BIT column.
func normalizeDamengAdditionalType(sqlType, additionalType string) string {
	if !strings.EqualFold(strings.TrimSpace(sqlType), "BIT") {
		return additionalType
	}
	return damengBooleanDefaultPattern.ReplaceAllStringFunc(additionalType, func(match string) string {
		if strings.HasSuffix(strings.ToLower(match), "true") {
			return "DEFAULT 1"
		}
		return "DEFAULT 0"
	})
}

func normalizeDamengDecimalPrecision(sqlType string) string {
	matches := damengDecimalTypePattern.FindStringSubmatch(sqlType)
	if matches == nil {
		return sqlType
	}

	precision, err := strconv.Atoi(matches[2])
	if err != nil {
		return sqlType
	}
	scale, err := strconv.Atoi(matches[3])
	if err != nil {
		return sqlType
	}
	if precision > 38 {
		precision = 38
	}
	if scale > precision {
		scale = precision
	}
	return fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)
}
