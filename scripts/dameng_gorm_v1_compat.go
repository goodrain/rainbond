package dm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var damengDecimalTypePattern = regexp.MustCompile(`(?i)^(decimal|numeric)\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)$`)

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
