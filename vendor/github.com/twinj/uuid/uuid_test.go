package uuid

/****************
 * Date: 3/02/14
 * Time: 10:59 PM
 ***************/

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

const (
	generate = 10000
)

var (
	goLang Name = "https://google.com/golang.org?q=golang"

	uuidBytes = []byte{
		0xaa, 0xcf, 0xee, 0x12,
		0xd4, 0x00,
		0x27, 0x23,
		0x00,
		0xd3,
		0x23, 0x12, 0x4a, 0x11, 0x89, 0xbb,
	}

	idString = "aacfee12-d400-2723-00d3-23124a1189bb"

	uuidVariants = []byte{
		VariantNCS, VariantRFC4122, VariantMicrosoft, VariantFuture,
	}

	namespaces = make(map[UUID]string)

	invalidHexStrings = [...]string{
		"foo",
		"6ba7b814-9dad-11d1-80b4-",
		"6ba7b814--9dad-11d1-80b4--00c04fd430c8",
		"6ba7b814-9dad7-11d1-80b4-00c04fd430c8999",
		"{6ba7b814-9dad-1180b4-00c04fd430c8",
		"{6ba7b814--11d1-80b4-00c04fd430c8}",
		"urn:uuid:6ba7b814-9dad-1666666680b4-00c04fd430c8",
	}

	validHexStrings = [...]string{
		"6ba7b8149dad-11d1-80b4-00c04fd430c8}",
		"{6ba7b8149dad-11d1-80b400c04fd430c8}",
		"{6ba7b814-9dad11d180b400c04fd430c8}",
		"6ba7b8149dad-11d1-80b4-00c04fd430c8",
		"6ba7b814-9dad11d1-80b4-00c04fd430c8",
		"6ba7b814-9dad-11d180b4-00c04fd430c8",
		"6ba7b814-9dad-11d1-80b400c04fd430c8",
		"6ba7b8149dad11d180b400c04fd430c8",
		"6ba7b814-9dad-11d1-80b4-00c04fd430c8",
		"{6ba7b814-9dad-11d1-80b4-00c04fd430c8}",
		"{6ba7b814-9dad-11d1-80b4-00c04fd430c8",
		"6ba7b814-9dad-11d1-80b4-00c04fd430c8}",
		"(6ba7b814-9dad-11d1-80b4-00c04fd430c8)",
		"urn:uuid:6ba7b814-9dad-11d1-80b4-00c04fd430c8",
	}
)

func init() {
	namespaces[NameSpaceX500] = "6ba7b814-9dad-11d1-80b4-00c04fd430c8"
	namespaces[NameSpaceOID] = "6ba7b812-9dad-11d1-80b4-00c04fd430c8"
	namespaces[NameSpaceURL] = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
	namespaces[NameSpaceDNS] = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	generator.init()
}

func TestEqual(t *testing.T) {
	for k, v := range namespaces {
		u, _ := Parse(v)
		assert.True(t, Equal(k, u), "Id's should be equal")
		assert.Equal(t, k.String(), u.String(), "Stringer versions should equal")
	}
}

func TestCompare(t *testing.T) {
	assert.True(t, Compare(NameSpaceDNS, NameSpaceDNS) == 0, "SDNS should be equal to DNS")
	assert.True(t, Compare(NameSpaceDNS, NameSpaceURL) == -1, "DNS should be less than URL")
	assert.True(t, Compare(NameSpaceURL, NameSpaceDNS) == 1, "URL should be greater than DNS")

	assert.True(t, Compare(nil, NameSpaceDNS) == -1, "Nil should be less than DNS")
	assert.True(t, Compare(NameSpaceDNS, nil) == 1, "DNS should be greater than Nil")
	assert.True(t, Compare(nil, nil) == 0, "nil should equal to nil")

	assert.True(t, Compare(Nil, NameSpaceDNS) == -1, "Nil should be less than DNS")
	assert.True(t, Compare(NameSpaceDNS, Nil) == 1, "DNS should be greater than Nil")
	assert.True(t, Compare(Nil, Nil) == 0, "Nil should equal to Nil")

	b1 := Uuid([]byte{
		0x01, 0x09, 0x09, 0x00,
		0xff, 0x02,
		0xff, 0x03,
		0x00,
		0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	b2 := Uuid([]byte{
		0x01, 0x09, 0x09, 0x00,
		0xff, 0x02,
		0xff, 0x03,
		0x00,
		0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	assert.Equal(t, 0, Compare(b1, b2), "Should equal")

	binary.BigEndian.PutUint32(b1[:4], 16779999)
	binary.BigEndian.PutUint32(b2[:4], 16780000)
	assert.Equal(t, -1, Compare(b1, b2), "Should be less")

	binary.BigEndian.PutUint32(b1[:4], 16780000)
	binary.BigEndian.PutUint32(b2[:4], 16779999)
	assert.Equal(t, 1, Compare(b1, b2), "Should be greater")

	binary.BigEndian.PutUint32(b2[:4], 16780000)
	assert.Equal(t, 0, Compare(b1, b2), "Should equal")

	binary.BigEndian.PutUint16(b1[4:6], 25000)
	binary.BigEndian.PutUint16(b2[4:6], 25001)
	assert.Equal(t, -1, Compare(b1, b2), "Should be less")

	binary.BigEndian.PutUint16(b1[4:6], 25001)
	binary.BigEndian.PutUint16(b2[4:6], 25000)
	assert.Equal(t, 1, Compare(b1, b2), "Should be greater")

	binary.BigEndian.PutUint16(b2[4:6], 25001)
	assert.Equal(t, 0, Compare(b1, b2), "Should equal")

	binary.BigEndian.PutUint16(b1[6:8], 25000)
	binary.BigEndian.PutUint16(b2[6:8], 25001)
	assert.Equal(t, -1, Compare(b1, b2), "Should be less")

	binary.BigEndian.PutUint16(b1[6:8], 25001)
	binary.BigEndian.PutUint16(b2[6:8], 25000)
	assert.Equal(t, 1, Compare(b1, b2), "Should be greater")

	binary.BigEndian.PutUint16(b2[6:8], 25001)
	assert.Equal(t, 0, Compare(b1, b2), "Should equal")

	b2[8] = 1
	assert.Equal(t, -1, Compare(b1, b2), "Should be less")

	b1[8] = 3
	assert.Equal(t, 1, Compare(b1, b2), "Should be greater")

}

func TestNewHex(t *testing.T) {
	s := "e902893a9d223c7ea7b8d6e313b71d9f"
	u := NewHex(s)
	assert.Equal(t, Three, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")

	assert.True(t, didNewHexPanic(), "Hex string should panic when invalid")
}

func didNewHexPanic() bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		NewHex("*********-------)()()()()(")
		return
	}()
}

func TestParse(t *testing.T) {
	for _, v := range invalidHexStrings {
		_, err := Parse(v)
		assert.Error(t, err, "Expected error due to invalid UUID string")
	}
	for _, v := range validHexStrings {
		_, err := Parse(v)
		assert.NoError(t, err, "Expected valid UUID string but got error")
	}
	for _, v := range namespaces {
		_, err := Parse(v)
		assert.NoError(t, err, "Expected valid UUID string but got error")
	}
}

func TestNew(t *testing.T) {
	for k := range namespaces {

		u := New(k.Bytes())

		assert.NotNil(t, u, "Expected a valid non nil UUID")
		assert.Equal(t, One, u.Version(), "Expected correct version %d, but got %d", One, u.Version())
		assert.Equal(t, VariantRFC4122, u.Variant(), "Expected ReservedNCS variant %x, but got %x", VariantNCS, u.Variant())
		assert.Equal(t, k.String(), u.String(), "Stringer versions should equal")
	}
}

func TestUUID_NewBulk(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		New(uuidBytes[:])
	}
}

func TestUUID_NewHexBulk(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		s := "f3593cffee9240df408687825b523f13"
		NewHex(s)
	}
}

func TestDigest(t *testing.T) {
	id := digest(md5.New(), []byte(NameSpaceDNS), goLang)
	u := Uuid(id)
	if u.Bytes() == nil {
		t.Error("Expected new data in bytes")
	}
	id = digest(sha1.New(), []byte(NameSpaceDNS), goLang)
	u = Uuid(id)
	if u.Bytes() == nil {
		t.Error("Expected new data in bytes")
	}
}

func TestNewV1(t *testing.T) {
	generator.Do(generator.init)
	u := NewV1()
	assert.Equal(t, One, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
}

func TestNewV2(t *testing.T) {
	u := NewV2(DomainGroup)

	assert.Equal(t, Two, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
}

func TestNewV3(t *testing.T) {
	u := NewV3(NameSpaceURL, goLang)

	assert.Equal(t, Three, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")

	ur, _ := url.Parse(string(goLang))

	// Same NS same name MUST be equal
	u2 := NewV3(NameSpaceURL, ur)
	assert.Equal(t, u, u2, "Expected UUIDs generated with same namespace and name to equal")

	// Different NS same name MUST NOT be equal
	u3 := NewV3(NameSpaceDNS, ur)
	assert.NotEqual(t, u, u3, "Expected UUIDs generated with different namespace and same name to be different")

	// Same NS different name MUST NOT be equal
	u4 := NewV3(NameSpaceURL, u)
	assert.NotEqual(t, u, u4, "Expected UUIDs generated with the same namespace and different names to be different")

	ids := []UUID{
		u, u2, u3, u4,
	}

	for j, id := range ids {
		i := NewV3(NameSpaceURL, Name(string(j)), id)
		assert.NotEqual(t, id, i, "Expected UUIDs generated with the same namespace and different names to be different")
	}

	u = NewV3(NameSpaceDNS, Name("www.example.com"))
	assert.Equal(t, "5df41881-3aed-3515-88a7-2f4a814cf09e", u.String())

	u = NewV3(NameSpaceDNS, Name("python.org"))
	assert.Equal(t, "6fa459ea-ee8a-3ca4-894e-db77e160355e", u.String())
}

func TestNewV4(t *testing.T) {
	u := NewV4()
	assert.Equal(t, Four, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
}

func TestNewV5(t *testing.T) {
	u := NewV5(NameSpaceURL, goLang)

	assert.Equal(t, Five, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")

	ur, _ := url.Parse(string(goLang))

	// Same NS same name MUST be equal
	u2 := NewV5(NameSpaceURL, ur)
	assert.Equal(t, u, u2, "Expected UUIDs generated with same namespace and name to equal")

	// Different NS same name MUST NOT be equal
	u3 := NewV5(NameSpaceDNS, ur)
	assert.NotEqual(t, u, u3, "Expected UUIDs generated with different namespace and same name to be different")

	// Same NS different name MUST NOT be equal
	u4 := NewV5(NameSpaceURL, u)
	assert.NotEqual(t, u, u4, "Expected UUIDs generated with the same namespace and different names to be different")

	ids := []UUID{
		u, u2, u3, u4,
	}

	for j, id := range ids {
		i := NewV5(NameSpaceURL, Name(string(j)), id)
		assert.NotEqual(t, i, id, "Expected UUIDs generated with the same namespace and different names to be different")
	}

	u = NewV5(NameSpaceDNS, Name("python.org"))
	assert.Equal(t, "886313e1-3b8a-5372-9b90-0c9aee199e5d", u.String())
}

var printIt = false

func printer(pId UUID) {
	if printIt {
		fmt.Println(pId)
	}
}

func TestUUID_NewV1Bulk(t *testing.T) {
	for i := 0; i < generate; i++ {
		printer(NewV1())
	}
}

func TestUUID_NewV3Bulk(t *testing.T) {
	for i := 0; i < generate; i++ {
		printer(NewV3(NameSpaceDNS, goLang, Name(string(i))))
	}
}

func TestUUID_NewV4Bulk(t *testing.T) {
	for i := 0; i < generate; i++ {
		printer(NewV4())
	}
}

func TestUUID_NewV5Bulk(t *testing.T) {
	for i := 0; i < generate; i++ {
		printer(NewV5(NameSpaceDNS, goLang, Name(string(i))))
	}
}

func Test_EachIsUnique(t *testing.T) {

	// Run half way through to avoid running within default resolution only

	spin := int(defaultSpinResolution / 2)

	for i := 0; i < spin; i++ {
		NewV1()
	}

	s := int(defaultSpinResolution)

	ids := make([]UUID, s)
	for i := 0; i < s; i++ {
		u := NewV1()
		ids[i] = u
		for j := 0; j < i; j++ {
			if b := assert.NotEqual(t, u.String(), ids[j].String(), "Should not create the same V1 UUID"); !b {
				break
			}
		}
	}
	//ids = make([]UUID, s)
	//for i := 0; i < s; i++ {
	//	u := NewV2(DomainGroup)
	//	ids[i] = u
	//	for j := 0; j < i; j++ {
	//		assert.NotEqual(t, u.String(), ids[j].String(), "Should not create the same V2 UUID")
	//	}
	//}
	ids = make([]UUID, s)
	for i := 0; i < s; i++ {
		u := NewV3(NameSpaceDNS, Name(string(i)), goLang)
		ids[i] = u
		for j := 0; j < i; j++ {
			if b := assert.NotEqual(t, u.String(), ids[j].String(), "Should not create the same V3 UUID"); !b {
				break
			}
		}
	}
	ids = make([]UUID, s)
	for i := 0; i < s; i++ {
		u := NewV4()
		ids[i] = u
		for j := 0; j < i; j++ {
			if b := assert.NotEqual(t, u.String(), ids[j].String(), "Should not create the same V4 UUID"); !b {
				break
			}
		}
	}
	ids = make([]UUID, s)
	for i := 0; i < s; i++ {
		u := NewV5(NameSpaceDNS, Name(string(i)), goLang)
		ids[i] = u
		for j := 0; j < i; j++ {
			if b := assert.NotEqual(t, u.String(), ids[j].String(), "Should not create the same V5 UUID"); !b {
				break
			}
		}
	}
}

func Test_NameSpaceUUIDs(t *testing.T) {
	for k, v := range namespaces {

		arrayId, _ := Parse(v)
		uuidId := array{}
		uuidId.unmarshal(arrayId.Bytes())
		assert.Equal(t, v, arrayId.String())
		assert.Equal(t, v, k.String())
	}
}

func TestNewV12(t *testing.T) {
	id := array{}

	makeUuid(&id,
		0x6ba7b810,
		0x9dad,
		0x11d1,
		0x80b4,
		[]byte{0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8})

	assert.Equal(t, id[:], NameSpaceDNS.Bytes())
	fmt.Println(Uuid(id[:]))
}
