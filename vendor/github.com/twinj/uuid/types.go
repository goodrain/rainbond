package uuid

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

const (
	length       = 16
	variantIndex = 8
	versionIndex = 6
)

// **************************************************** Create UUIDs

type array [length]byte

func (o *array) unmarshal(pData []byte) {
	copy(o[:], pData)
}

// Set the three most significant bits (bits 0, 1 and 2) of the
// sequenceHiAndVariant equivalent in the array to ReservedRFC4122.
func (o *array) setRFC4122Version(pVersion uint8) {
	o[versionIndex] &= 0x0f
	o[versionIndex] |= uint8(pVersion << 4)
	o[variantIndex] &= variantSet
	o[variantIndex] |= VariantRFC4122
}

// **************************************************** Default implementation

var _ UUID = &Uuid{}

// Uuid is the default UUID implementation. All uuid functions will return this
// type. All uuid functions should use UUID as their in parameter.
type Uuid []byte

// Size returns the octet length of the Uuid
func (o Uuid) Size() int {
	return length
}

// Version returns the uuid.Version of the Uuid
func (o Uuid) Version() Version {
	return resolveVersion(o[versionIndex] >> 4)
}

// Variant returns the implementation variant of the Uuid
func (o Uuid) Variant() uint8 {
	return variant(o[variantIndex])
}

// Bytes return the underlying data representation of the Uuid in network byte
// order
func (o Uuid) Bytes() []byte {
	return o
}

// String returns the canonical string representation of the UUID or the
// uuid.Format the package is set to via uuid.SwitchFormat
func (o Uuid) String() string {
	return formatUuid(o, printFormat)
}

// **************************************************** Implementations

// MarshalBinary implements the encoding.BinaryMarshaler interface
func (o Uuid) MarshalBinary() ([]byte, error) {
	return o.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (o *Uuid) UnmarshalBinary(pBytes []byte) error {
	if len(pBytes) != o.Size() {
		return fmt.Errorf("uuid.Uuid.UnmarshalBinary:  invalid length")
	}
	if len(*o) != 0 {
		panic("uuid.Uuid.UnmarshalBinary: you must use an empty or new Uuid to unmarhal bytes")
	}
	*o = append(*o, pBytes...)
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface. It will marshal
// text into one of the known formats, if you have changed to a custom Format
// the text
func (o Uuid) MarshalText() ([]byte, error) {
	f := FormatCanonical
	if defaultFormats[printFormat] {
		f = printFormat
	}
	return []byte(strings.ToLower(string(format(o.Bytes(), string(f))))), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface. It will
// support any text that MarshalText can produce.
func (o *Uuid) UnmarshalText(pUuid []byte) error {
	id, err := parse(string(pUuid))
	if err == nil {
		o.UnmarshalBinary(id)
	}
	return err
}

// Value implements the driver.Valuer interface
func (o Uuid) Value() (value driver.Value, err error) {
	if len(o) == 0 {
		value, err = nil, nil
		return
	}
	value, err = o.MarshalText()
	return
}

// Scan implements the sql.Scanner interface
func (o *Uuid) Scan(pSrc interface{}) error {
	if pSrc == nil {
		return nil
	}
	if pSrc == "" {
		return nil
	}
	switch src := pSrc.(type) {

	case string:
		return o.UnmarshalText([]byte(src))

	case []byte:
		if len(src) == length {
			return o.UnmarshalBinary(src)
		} else {
			return o.UnmarshalText(src)
		}

	default:
		return fmt.Errorf("uuid.Uuid.Scan: cannot scan type %T into Uuid", pSrc)
	}
}

// **************************************************** Immutable UUID

var _ UUID = new(Immutable)

// Immutable is an easy to use UUID which can be used as a key or for constants
type Immutable string

// Size returns the octet length of the Uuid
func (o Immutable) Size() int {
	return length
}

// Version returns the uuid.Version of the Uuid
func (o Immutable) Version() Version {
	return resolveVersion(o[versionIndex] >> 4)
}

// Variant returns the implementation variant of the Uuid
func (o Immutable) Variant() uint8 {
	return variant(o[variantIndex])
}

// Bytes return the underlying data representation of the Uuid in network byte
// order
func (o Immutable) Bytes() []byte {
	return Uuid(o).Bytes()
}

// String returns the canonical string representation of the UUID or the
// uuid.Format the package is set to via uuid.SwitchFormat
func (o Immutable) String() string {
	return Uuid(o).String()
}
