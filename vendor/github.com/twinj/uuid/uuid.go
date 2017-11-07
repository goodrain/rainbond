// This package provides RFC4122 and DCE 1.1 UUIDs.
//
// Use NewV1, NewV2, NewV3, NewV4, NewV5, for generating new UUIDs.
//
// Use New([]byte), NewHex(string), and Parse(string) for
// creating UUIDs from existing data.
//
// If you have a []byte you can simply cast it to the Uuid type.
//
// The original version was from Krzysztof Kowalik <chris@nu7hat.ch>
// Unfortunately, that version was non compliant with RFC4122.
//
// The package has since been redesigned.
//
// The example code in the specification was also used as reference
// for design.
//
// Copyright (C) 2016 twinj@github.com  2016 MIT licence
package uuid

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"log"
	"regexp"
)

const (
	Nil Immutable = "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

	// The following standard UUIDs are for use with V3 or V5 UUIDs.
	NameSpaceDNS  Immutable = "k\xa7\xb8\x10\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceURL  Immutable = "k\xa7\xb8\x11\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceOID  Immutable = "k\xa7\xb8\x12\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceX500 Immutable = "k\xa7\xb8\x14\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
)

type Domain uint8

const (
	DomainUser Domain = iota + 1
	DomainGroup
)

// UUID is the common interface implemented by all UUIDs
type UUID interface {

	// Bytes retrieves the bytes from the underlying UUID
	Bytes() []byte

	// Size is the length of the underlying UUID implementation
	Size() int

	// String should return the canonical UUID representation, or the a
	// given uuid.Format
	String() string

	// Variant returns the UUID implementation variant
	Variant() uint8

	// Version returns the version number of the algorithm used to generate
	// the UUID.
	Version() Version
}

// New creates a UUID from a slice of bytes.
func New(pData []byte) Uuid {
	o := array{}
	o.unmarshal(pData)
	return o[:]
}

// NewHex creates a UUID from a hex string
// Will panic if hex string is invalid use Parse otherwise.
func NewHex(pUuid string) Uuid {
	return Uuid(fromHex(pUuid))
}

const (
	// Pattern used to parse string representation of the UUID.
	// Current one allows to parse string where only one opening
	// or closing bracket or any of the hyphens are optional.
	// It is only used to extract the main bytes to create a UUID,
	// so these imperfections are of no consequence.
	hexPattern = `^(urn\:uuid\:)?[\{\(\[]?([[:xdigit:]]{8})-?([[:xdigit:]]{4})-?([1-5][[:xdigit:]]{3})-?([[:xdigit:]]{4})-?([[:xdigit:]]{12})[\]\}\)]?$`
)

var (
	parseUUIDRegex = regexp.MustCompile(hexPattern)
)

// Parse creates a UUID from a valid string representation.
// Accepts UUID string in following formats:
//		6ba7b8149dad11d180b400c04fd430c8
//		6ba7b814-9dad-11d1-80b4-00c04fd430c8
//		{6ba7b814-9dad-11d1-80b4-00c04fd430c8}
//		urn:uuid:6ba7b814-9dad-11d1-80b4-00c04fd430c8
//		[6ba7b814-9dad-11d1-80b4-00c04fd430c8]
//		(6ba7b814-9dad-11d1-80b4-00c04fd430c8)
//
func Parse(pUuid string) (Uuid, error) {
	id, err := parse(pUuid)
	return Uuid(id), err
}

func parse(pUuid string) ([]byte, error) {
	md := parseUUIDRegex.FindStringSubmatch(pUuid)
	if md == nil {
		return nil, errors.New("uuid.Parse: invalid string format this is probably not a UUID")
	}
	return fromHex(md[2] + md[3] + md[4] + md[5] + md[6]), nil
}

func fromHex(pUuid string) []byte {
	bytes, err := hex.DecodeString(pUuid)
	if err != nil {
		panic(err)
	}
	return bytes
}

// NewV1 generates a new RFC4122 version 1 UUID based on a 60 bit timestamp and
// node ID.
func NewV1() Uuid {
	return generator.NewV1()
}

// NewV2 generates a new DCE Security version UUID based on a 60 bit timestamp,
// node id and POSIX UID.
func NewV2(pDomain Domain) Uuid {
	return generator.NewV2(pDomain)
}

// NewV3 generates a new RFC4122 version 3 UUID based on the MD5 hash on a
// namespace UUID and any type which implements the UniqueName interface
// for the name. For strings and slices cast to a Name type
func NewV3(pNamespace UUID, pNames ...UniqueName) Uuid {
	o := array{}
	copy(o[:], digest(md5.New(), pNamespace.Bytes(), pNames...))
	o.setRFC4122Version(3)
	return o[:]
}

// NewV4 generates a new RFC4122 version 4 UUID a cryptographically secure
// random UUID.
func NewV4() Uuid {
	o, err := v4()
	if err == nil {
		return o[:]
	}
	generator.err = err
	log.Printf("uuid.V4: There was an error getting random bytes [%s]\n", err)
	if ok := generator.HandleError(err); ok {
		o, err = v4()
		if err == nil {
			return o[:]
		}
		generator.err = err
	}
	return nil
}

func v4() (o array, err error) {
	generator.err = nil
	_, err = generator.Random(o[:])
	o.setRFC4122Version(4)
	return
}

// NewV5 generates an RFC4122 version 5 UUID based on the SHA-1 hash of a
// namespace UUID and a unique name.
func NewV5(pNamespace UUID, pNames ...UniqueName) Uuid {
	o := array{}
	copy(o[:], digest(sha1.New(), pNamespace.Bytes(), pNames...))
	o.setRFC4122Version(5)
	return o[:]
}

func digest(pHash hash.Hash, pName []byte, pNames ...UniqueName) []byte {
	for _, v := range pNames {
		pName = append(pName, v.String()...)
	}
	pHash.Write(pName)
	return pHash.Sum(nil)
}

// Compare returns an integer comparing two UUIDs lexicographically.
// The result will be 0 if pId==pId2, -1 if pId < pId2, and +1 if pId > pId2.
// A nil argument is equivalent to the Nil UUID.
func Compare(pId, pId2 UUID) int {

	var b1, b2 []byte

	if pId == nil {
		b1 = []byte(Nil)
	} else {
		b1 = pId.Bytes()
	}

	if pId2 == nil {
		b2 = []byte(Nil)
	} else {
		b2 = pId2.Bytes()
	}

	tl1 := binary.BigEndian.Uint32(b1[:4])
	tl2 := binary.BigEndian.Uint32(b2[:4])

	if tl1 != tl2 {
		if tl1 < tl2 {
			return -1
		} else {
			return 1
		}
	}

	m1 := binary.BigEndian.Uint16(b1[4:6])
	m2 := binary.BigEndian.Uint16(b2[4:6])

	if m1 != m2 {
		if m1 < m2 {
			return -1
		} else {
			return 1
		}
	}

	m1 = binary.BigEndian.Uint16(b1[6:8])
	m2 = binary.BigEndian.Uint16(b2[6:8])

	if m1 != m2 {
		if m1 < m2 {
			return -1
		} else {
			return 1
		}
	}

	return bytes.Compare(b1[8:], b2[8:])
}

// Compares whether each UUID is the same
func Equal(p1, p2 UUID) bool {
	return bytes.Equal(p1.Bytes(), p2.Bytes())
}

// Name is a string which implements UniqueName and satisfies the Stringer
// interface. V3 and V5 UUIDs use this for hashing values together to produce
// UUIDs based on a NameSpace.
type Name string

// String returns the uuid.Name as a string.
func (o Name) String() string {
	return string(o)
}

// UniqueName is a Stinger interface made for easy passing of any Stringer type
// into a Hashable UUID.
type UniqueName interface {
	// Many go types implement this method for use with printing
	// Will convert the current type to its native string format
	String() string
}
