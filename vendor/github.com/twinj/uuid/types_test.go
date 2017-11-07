package uuid

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUuid_Bytes(t *testing.T) {
	id := make(Uuid, length)
	copy(id, NameSpaceDNS.Bytes())
	assert.Equal(t, id.Bytes(), NameSpaceDNS.Bytes(), "Bytes should be the same")
}

func TestUuid_Size(t *testing.T) {
	id := make(Uuid, length)
	assert.Equal(t, 16, id.Size(), "The size of the array should be sixteen")
}

func TestUuid_String(t *testing.T) {
	id := Uuid(uuidBytes)
	assert.Equal(t, idString, id.String(), "The Format given should match the output")
}

func TestUuid_Variant(t *testing.T) {
	bytes := make(Uuid, length)
	copy(bytes, uuidBytes[:])

	for _, v := range uuidVariants {
		for i := 0; i <= 255; i++ {
			bytes[variantIndex] = byte(i)
			id := createMarshaler(bytes, 4, v)
			b := id[variantIndex] >> 4
			tVariantConstraint(v, b, id, t)
			assert.Equal(t, v, id.Variant(), "%x does not resolve to %x", id.Variant(), v)
		}
	}

	assert.True(t, didMarshalerSetVariantPanic(bytes[:]), "Array creation should panic  if invalid variant")
}

func didMarshalerSetVariantPanic(bytes []byte) bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		createMarshaler(bytes[:], 4, 0xbb)
		return
	}()
}

func TestUuid_Version(t *testing.T) {
	id := make(Uuid, length)

	bytes := make(Uuid, length)
	copy(bytes, uuidBytes[:])

	assert.Equal(t, Unknown, id.Version(), "The version should be 0")

	for v := 0; v < 16; v++ {
		for i := 0; i <= 255; i++ {
			bytes[versionIndex] = byte(i)
			copy(id, bytes)
			setVersion(&id[versionIndex], v)
			if v > 0 && v < 6 {
				assert.Equal(t, Version(v), id.Version(), "%x does not resolve to %x", id.Version(), v)
			} else {
				assert.Equal(t, Version(v), getVersion(id), "%x does not resolve to %x", getVersion(id), v)
			}
		}
	}
}

func TestImmutable_Bytes(t *testing.T) {
	b := make([]byte, length)
	copy(b[:], NameSpaceDNS.Bytes())

	id := Immutable(b)

	assert.Equal(t, NameSpaceDNS.Bytes(), id.Bytes())
}

func TestImmutable_Size(t *testing.T) {
	assert.Equal(t, 16, Nil.Size(), "The size of the array should be sixteen")
}

func TestImmutable_String(t *testing.T) {
	id := Immutable(uuidBytes)
	assert.Equal(t, idString, id.String(), "The Format given should match the output")
}

func TestImmutable_Variant(t *testing.T) {
	bytes := make(Uuid, length)
	copy(bytes, uuidBytes[:])

	for _, v := range uuidVariants {
		for i := 0; i <= 255; i++ {
			bytes[variantIndex] = byte(i)
			id := createMarshaler(bytes, 4, v)
			b := id[variantIndex] >> 4
			tVariantConstraint(v, b, id, t)
			id2 := Immutable(id)
			assert.Equal(t, v, id2.Variant(), "%x does not resolve to %x", id2.Variant(), v)
		}
	}
}

func TestImmutable_Version(t *testing.T) {

	id := make(Uuid, length)
	bytes := make(Uuid, length)
	copy(bytes, uuidBytes[:])

	for v := 0; v < 16; v++ {
		for i := 0; i <= 255; i++ {
			bytes[versionIndex] = byte(i)
			copy(id, bytes)
			setVersion(&id[versionIndex], v)
			id2 := Immutable(id)

			if v > 0 && v < 6 {
				assert.Equal(t, Version(v), id2.Version(), "%x does not resolve to %x", id2.Version(), v)
			} else {
				assert.Equal(t, Version(v), getVersion(Uuid(id)), "%x does not resolve to %x", getVersion(Uuid(id)), v)
			}
		}
	}
}

func TestUuid_MarshalBinary(t *testing.T) {
	id := Uuid(uuidBytes)
	bytes, err := id.MarshalBinary()
	assert.Nil(t, err, "There should be no error")
	assert.Equal(t, uuidBytes[:], bytes, "Byte should be the same")
}

func TestUuid_UnmarshalBinary(t *testing.T) {

	assert.True(t, didUnmarshalPanic(), "Should panic")

	u := Uuid{}
	err := u.UnmarshalBinary([]byte{1, 2, 3, 4, 5})

	assert.Error(t, err, "Expect length error")

	err = u.UnmarshalBinary(uuidBytes[:])

	u = Uuid{}

	err = u.UnmarshalBinary(uuidBytes[:])

	assert.Nil(t, err, "There should be no error but got %s", err)

	for k, v := range namespaces {
		id, _ := Parse(v)
		u = Uuid{}
		u.UnmarshalBinary(id.Bytes())

		assert.Equal(t, id.Bytes(), u.Bytes(), "The array id should equal the uuid id")
		assert.Equal(t, k.Bytes(), u.Bytes(), "The array id should equal the uuid id")
	}
}

func didUnmarshalPanic() bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()
		u := make(Uuid, length)
		u.UnmarshalBinary(uuidBytes[:])
		return
	}()
}

func TestUuid_Scan(t *testing.T) {
	var v Uuid
	assert.Nil(t, v)

	err := v.Scan(nil)
	assert.NoError(t, err, "When nil there should be no error")
	assert.Empty(t, v, "Should have no data")

	err = v.Scan("")
	assert.NoError(t, err, "When nil there should be no error")
	assert.Empty(t, v, "Should have no data")

	var v2 Uuid
	err = v2.Scan(NameSpaceDNS.Bytes())
	assert.NoError(t, err, "When nil there should be no error")
	assert.Equal(t, NameSpaceDNS.Bytes(), v2.Bytes(), "Values should be the same")

	err = v.Scan(NameSpaceDNS.String())
	assert.NoError(t, err, "When nil there should be no error")
	assert.Equal(t, NameSpaceDNS.String(), v.String(), "Values should be the same")

	var v3 Uuid
	err = v3.Scan([]byte(NameSpaceDNS.String()))
	assert.NoError(t, err, "When []byte represents string should be no error")
	assert.Equal(t, NameSpaceDNS.String(), v3.String(), "Values should be the same")

	err = v.Scan(22)
	assert.Error(t, err, "When wrong type should error")
}

func TestUuid_Value(t *testing.T) {
	var v Uuid
	assert.Nil(t, v)

	id, err := v.Value()
	assert.Nil(t, id, "There hsould be no driver valuie")
	assert.NoError(t, err, "There should be no error")

	ns := Uuid(NameSpaceDNS)

	id, err = ns.Value()
	assert.NotNil(t, id, "Ther hsould be a vliad driver value")
	assert.NoError(t, err, "There should be no error")
}

func getVersion(pId Uuid) Version {
	return Version(pId[versionIndex] >> 4)
}

func createMarshaler(pData []byte, pVersion int, pVariant uint8) Uuid {
	o := make(Uuid, length)
	copy(o, pData)
	setVersion(&o[versionIndex], pVersion)
	setVariant(&o[variantIndex], pVariant)
	return o
}

func setVersion(pByte *byte, pVersion int) {
	*pByte &= 0x0f
	*pByte |= uint8(pVersion << 4)
}

func setVariant(pByte *byte, pVariant uint8) {
	switch pVariant {
	case VariantRFC4122:
		*pByte &= variantSet
	case VariantFuture, VariantMicrosoft:
		*pByte &= 0x1F
	case VariantNCS:
		*pByte &= 0x7F
	default:
		panic(errors.New("uuid.setVariant: invalid variant mask"))
	}
	*pByte |= pVariant
}
