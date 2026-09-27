package parser

import "strings"

// TarImageEventID recognizes only an owned event directory identifier, never a
// caller-supplied filesystem path or a malformed event command.
func TarImageEventID(source string) (string, bool) {
	fields := strings.Fields(source)
	if !strings.HasPrefix(source, "event") || len(fields) != 2 || fields[0] != "event" || len(fields[1]) == 0 || len(fields[1]) > 128 {
		return "", false
	}
	for _, r := range fields[1] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", false
		}
	}
	return fields[1], true
}
