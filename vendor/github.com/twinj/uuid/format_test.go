package uuid

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

const (
	clean                   = `[0-9a-f]{8}[0-9a-f]{4}[1-5][0-9a-f]{3}[0-9a-f]{4}[0-9a-f]{12}`
	cleanHexPattern         = `^` + clean + `$`
	curlyHexPattern         = `^\{` + clean + `\}$`
	bracketHexPattern       = `^\(` + clean + `\)$`
	hyphen                  = `[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`
	cleanHyphenHexPattern   = `^` + hyphen + `$`
	curlyHyphenHexPattern   = `^\{` + hyphen + `\}$`
	bracketHyphenHexPattern = `^\(` + hyphen + `\)$`
	urnHexPattern           = `^urn:uuid:` + hyphen + `$`
)

var (
	formats = []Format{
		FormatCanonicalCurly,
		FormatHex,
		FormatHexCurly,
		FormatHexBracket,
		FormatCanonical,
		FormatCanonicalBracket,
		FormatUrn,
	}
	patterns = []string{
		curlyHyphenHexPattern,
		cleanHexPattern,
		curlyHexPattern,
		bracketHexPattern,
		cleanHyphenHexPattern,
		bracketHyphenHexPattern,
		urnHexPattern,
	}
)

func TestSwitchFormat(t *testing.T) {
	ids := []UUID{NewV4(), NewV4()}

	// Reset default
	SwitchFormat(FormatCanonical)

	for _, u := range ids {
		for i := range formats {
			SwitchFormat(formats[i])
			assert.True(t, regexp.MustCompile(patterns[i]).MatchString(u.String()), "Format %s must compile pattern %s", formats[i], patterns[i])
		}
	}

	assert.True(t, didSwitchFormatPanic(""), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatPanic("%c%c%c%x%x%x"), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatPanic("%x%X%x"), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatPanic("%x%x%x%x%x%%%%"), "Switch format should panic when format invalid")

	// Reset default
	SwitchFormat(FormatCanonical)
}

func didSwitchFormatPanic(pFormat string) bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		SwitchFormat(Format(pFormat))
		return
	}()
}

func TestSwitchFormatToUpper(t *testing.T) {
	ids := []UUID{NewV4(), NewV4()}

	// Reset default
	SwitchFormat(FormatCanonical)

	for _, u := range ids {
		for i := range formats {
			SwitchFormatToUpper(formats[i])
			assert.True(t, regexp.MustCompile(strings.ToUpper(patterns[i])).MatchString(u.String()), "Format %s must compile pattern %s", formats[i], patterns[i])
		}
	}

	assert.True(t, didSwitchFormatToUpperPanic(""), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatToUpperPanic("%c%c%c%x%x%x"), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatToUpperPanic("%x%X%x"), "Switch format should panic when format invalid")
	assert.True(t, didSwitchFormatToUpperPanic("%x%x%x%x%x%%%%"), "Switch format should panic when format invalid")

	// Reset default
	SwitchFormat(FormatCanonical)
}

func didSwitchFormatToUpperPanic(pFormat string) bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		SwitchFormatToUpper(Format(pFormat))
		return
	}()
}

func TestFormatter(t *testing.T) {
	ids := []UUID{NewV4(), NewV4()}

	for _, u := range ids {
		for i := range formats {
			assert.True(t, regexp.MustCompile(patterns[i]).MatchString(Formatter(u, formats[i])), "Format must compile")
		}
	}

	for k, v := range namespaces {
		s := Formatter(k, FormatCanonical)
		assert.Equal(t, v, s, "Should match")

		s = Formatter(k, Format(strings.ToUpper(string(FormatCanonical))))
		assert.Equal(t, strings.ToUpper(v), s, "Should match")
	}

	assert.True(t, didFormatterPanic(""), "Should panic when format invalid")
	assert.True(t, didFormatterPanic("%c%c%c%x%x%x"), "Should panic when format invalid")
	assert.True(t, didFormatterPanic("%x%X%x"), "Should panic when format invalid")
	assert.True(t, didFormatterPanic("%x%x%x%x%x%%%%"), "Should panic when format invalid")

}

func didFormatterPanic(pFormat string) bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		Formatter(NameSpaceDNS, Format(pFormat))
		return
	}()
}
