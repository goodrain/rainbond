package uuid

import (
	"errors"
	"strings"
)

// Format represents different styles a UUID can be printed in constants
// represent a pattern used by the package with which to print a UUID.
type Format string

const (
	FormatHex        Format = "%x%x%x%x%x"
	FormatHexCurly   Format = "{%x%x%x%x%x}"
	FormatHexBracket Format = "(%x%x%x%x%x)"

	// This is the canonical format.
	FormatCanonical Format = "%x-%x-%x-%x-%x"

	FormatCanonicalCurly   Format = "{%x-%x-%x-%x-%x}"
	FormatCanonicalBracket Format = "(%x-%x-%x-%x-%x)"
	FormatUrn              Format = "urn:uuid:" + FormatCanonical
)

var printFormat Format = FormatCanonical

var defaultFormats map[Format]bool = make(map[Format]bool)

func init() {
	defaultFormats[FormatHex] = true
	defaultFormats[FormatHexCurly] = true
	defaultFormats[FormatHexBracket] = true
	defaultFormats[FormatCanonical] = true
	defaultFormats[FormatCanonicalCurly] = true
	defaultFormats[FormatCanonicalBracket] = true
	defaultFormats[FormatUrn] = true
}

// SwitchFormat switches the default printing format for ALL UUIDs.
//
// The default is the canonical uuid.Format.FormatCanonical which has been
// optimised for use with this package. It is twice as fast compared to other
// formats; supplied or given. However, the benchmark for non default formats
// is still very quick and quite usable. The package has moved away from using
// fmt.Sprintf which was up to 5 times slower in comparison to custom formats
// and 10 times slower in comparison to the canonical format.
//
// A valid format will have 5 groups of [%x|%X] or follow the pattern,
// *%[xX]*%[xX]*%[xX]*%[xX]*%[xX]*. If the supplied format does not meet this
// standard the function will panic. Note any extra uses of [%] outside of the
// [%x|%X] will also cause a panic.
// Constant uuid.Formats have been provided for the most likely formats.
func SwitchFormat(pFormat Format) {
	checkFormat(pFormat)
	printFormat = pFormat
}

// SwitchFormatToUpper is a convenience function to set the Format to uppercase
// versions of the given constants.
func SwitchFormatToUpper(pFormat Format) {
	SwitchFormat(Format(strings.ToUpper(string(pFormat))))
}

// Formatter will return a string representation of the given UUID.
//
// Use this for one time formatting when setting the default using
// uuid.SwitchFormat would be overkill.
//
// A valid format will have 5 groups of [%x|%X] or follow the pattern,
// *%[xX]*%[xX]*%[xX]*%[xX]*%[xX]*. If the supplied format does not meet this
// standard the function will panic. Note any extra uses of [%] outside of the
// [%x|%X] will also cause a panic.
func Formatter(pId UUID, pFormat Format) string {
	checkFormat(pFormat)
	return formatUuid(pId.Bytes(), pFormat)
}

func checkFormat(pFormat Format) {
	if defaultFormats[pFormat] {
		return
	}
	s := strings.ToLower(string(pFormat))
	if strings.Count(s, "%x") != 5 {
		panic(errors.New("uuid.Format: invalid format"))
	}
	s = strings.Replace(s, "%x", "", -1)
	if strings.Count(s, "%") > 0 {
		panic(errors.New("uuid.Format: invalid format"))
	}
}

const (
	hexTable      = "0123456789abcdef"
	hexUpperTable = "0123456789ABCDEF"

	canonicalLength      = length*2 + 4
	formatArgCount       = 10
	uuidStringBufferSize = length*2 - formatArgCount
)

var groups = [...]int{4, 2, 2, 2, 6}

func formatUuid(pSrc []byte, pFormat Format) string {
	if pFormat == FormatCanonical {
		return string(formatCanonical(pSrc))
	}
	return string(format(pSrc, string(pFormat)))
}

func format(pSrc []byte, pFormat string) []byte {
	end := len(pFormat)
	buf := make([]byte, end+uuidStringBufferSize)

	var s, ls, b, e, p int
	var u bool
	for _, v := range groups {
		ls = s
		for ; s < end && pFormat[s] != '%'; s++ {
		}
		copy(buf[p:], pFormat[ls:s])
		p += s - ls
		s++
		u = pFormat[s] == 'X'
		s++
		e = b + v
		for i, t := range pSrc[b:e] {
			j := p + i + i
			table := hexTable
			if u {
				table = hexUpperTable
			}
			buf[j] = table[t>>4]
			buf[j+1] = table[t&0x0f]
		}
		b = e
		p += v + v
	}
	ls = s
	for ; s < end && pFormat[s] != '%'; s++ {
	}
	copy(buf[p:], pFormat[ls:s])
	p += s - ls
	return buf
}

func formatCanonical(pSrc []byte) []byte {
	buf := make([]byte, canonicalLength)
	var b, p, e int
	for h, v := range groups {
		e = b + v
		for i, t := range pSrc[b:e] {
			j := p + i + i
			buf[j] = hexTable[t>>4]
			buf[j+1] = hexTable[t&0x0f]
		}
		b = e
		p += v + v
		if h < 4 {
			buf[p] = '-'
			p += 1
		}
	}
	return buf
}
