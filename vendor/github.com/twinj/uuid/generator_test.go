package uuid

import (
	"crypto/rand"
	"errors"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

var (
	nodeBytes = []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb, 0x44, 0xcc}
)

func init() {
	generator.init()
}

func TestGenerator_V1(t *testing.T) {
	u := generator.NewV1()

	assert.Equal(t, One, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
}

func TestGenerator_V2(t *testing.T) {
	u := generator.NewV2(DomainGroup)

	assert.Equal(t, Two, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
	assert.Equal(t, uint8(DomainGroup), u.Bytes()[9], "Expected string representation to be valid")

	u = generator.NewV2(DomainUser)

	assert.Equal(t, Two, u.Version(), "Expected correct version")
	assert.Equal(t, VariantRFC4122, u.Variant(), "Expected correct variant")
	assert.True(t, parseUUIDRegex.MatchString(u.String()), "Expected string representation to be valid")
	assert.Equal(t, uint8(DomainUser), u.Bytes()[9], "Expected string representation to be valid")
}

func TestRegisterGenerator(t *testing.T) {
	g1 := GeneratorConfig{
		nil,
		func() Timestamp {
			return Timestamp(145876)
		}, 0,
		func() Node {
			return []byte{0x11, 0xaa, 0xbb, 0xaa, 0xbb, 0xcc}
		},
		func([]byte) (int, error) {
			return 58, nil
		}, nil}

	once = new(sync.Once)
	RegisterGenerator(g1)

	assert.Equal(t, g1.Next(), generator.Next(), "These values should be the same")
	assert.Equal(t, g1.Id(), generator.Id(), "These values should be the same")

	n1, err1 := generator.Random(nil)
	n, err := generator.Random(nil)
	assert.Equal(t, n1, n, "Values should be the same")
	assert.Equal(t, err, err1, "Values should be the same")
	assert.NoError(t, err)

	assert.True(t, didRegisterGeneratorPanic(g1), "Should panic when invalid")
}

func didRegisterGeneratorPanic(gen GeneratorConfig) bool {
	return func() (didPanic bool) {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		RegisterGenerator(gen)
		return
	}()
}

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator(GeneratorConfig{})

	assert.NotNil(t, gen.Next, "There shoud be a default Next function")
	assert.NotNil(t, gen.Random, "There shoud be a default Random function")
	assert.NotNil(t, gen.HandleError, "There shoud be a default HandleError function")
	assert.NotNil(t, gen.Id, "There shoud be a default Id function")

	assert.Equal(t, findFirstHardwareAddress(), gen.Id(), "There shoud be the gieen Id function")

	gen = NewGenerator(GeneratorConfig{
		Id: func() Node {
			return Node{0xaa, 0xff}
		},
		Next: func() Timestamp {
			return Timestamp(2)
		},
		HandleError: func(error) bool {
			return true
		},
		Random: func([]byte) (int, error) {
			return 1, nil
		},
		Resolution: 0,
	})

	assert.NotNil(t, gen.Next, "There shoud be a default Next function")
	assert.NotNil(t, gen.Random, "There shoud be a default Random function")
	assert.NotNil(t, gen.HandleError, "There shoud be a default HandleError function")
	assert.NotNil(t, gen.Id, "There shoud be a default Id function")

	n, err := gen.Random(nil)

	assert.Equal(t, Timestamp(2), gen.Next(), "There shoud be the given Next function")
	assert.Equal(t, 1, n, "There shoud be the given Random function")
	assert.NoError(t, err, "There shoud be the given Random function")
	assert.Equal(t, true, gen.HandleError(nil), "There shoud be the given HandleError function")
	assert.Equal(t, Node{0xaa, 0xff}, gen.Id(), "There shoud be the gieen Id function")

	gen = NewGenerator(GeneratorConfig{
		Id: func() Node {
			return []byte{0xaa, 0xff}
		},
		Next: nil,
		HandleError: func(error) bool {
			return true
		},
		Random: func([]byte) (int, error) {
			return 1, nil
		},
		Resolution: 4096,
	})

	n, err = gen.Random(nil)

	assert.NotNil(t, gen.Next, "There shoud be a default Next function")
	assert.NotNil(t, gen.Random, "There shoud be a default Random function")
	assert.NotNil(t, gen.HandleError, "There shoud be a default HandleError function")
	assert.NotNil(t, gen.Id, "There shoud be a default Id function")

	assert.Equal(t, 1, n, "There shoud be the given Random function")
	assert.NoError(t, err, "There shoud be the given Random function")
	assert.Equal(t, true, gen.HandleError(nil), "There shoud be the given HandleError function")
	assert.Equal(t, Node{0xaa, 0xff}, gen.Id(), "There shoud be the gieen Id function")

}

func TestGeneratorInit(t *testing.T) {
	// A new time that is older than stored time should cause the sequence to increment
	now, node := registerTestGenerator(Now(), nodeBytes)
	storageStamp := registerSaver(now.Add(time.Second), node)

	assert.NotNil(t, generator.Store, "Generator should not return an empty store")
	assert.True(t, generator.Timestamp < storageStamp, "Increment sequence when old timestamp newer than new")
	assert.Equal(t, Sequence(3), generator.Sequence, "Successfull read should have incremented sequence")

	// Nodes not the same should generate a random sequence
	now, node = registerTestGenerator(Now(), nodeBytes)
	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.NotNil(t, generator.Store, "Generator should not return an empty store")
	assert.True(t, generator.Timestamp > storageStamp, "New timestamp should be newer than old")
	assert.NotEqual(t, Sequence(2), generator.Sequence, "Sequence should not be same as storage")
	assert.NotEqual(t, Sequence(3), generator.Sequence, "Sequence should not be incremented but be random")
	assert.Equal(t, generator.Node, node, generator.Sequence, "Node should be equal")

	now, node = registerTestGenerator(Now(), nodeBytes)

	// Random read error should alert user
	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.Error(t, generator.err, "Read error should exist")

	now, node = registerTestGenerator(Now(), nil)
	// Random read error should alert user
	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	storageStamp = registerSaver(now.Sub(time.Second), []byte{0xaa, 0xee, 0xaa, 0xbb, 0x44, 0xcc})

	assert.Error(t, generator.Error(), "Read error should exist")

	registerDefaultGenerator()
}

func TestGeneratorRead(t *testing.T) {
	// A new time that is older than stored time should cause the sequence to increment
	now := Now()
	i := 0

	timestamps := []Timestamp{
		now.Sub(time.Second),
		now.Sub(time.Second * 2),
	}

	generator = NewGenerator(GeneratorConfig{
		nil,
		func() Timestamp {
			return timestamps[i]
		}, 0,
		func() Node {
			return nodeBytes
		},
		rand.Read,
		nil})

	storageStamp := registerSaver(now.Add(time.Second), nodeBytes)

	i++

	generator.read()

	assert.True(t, generator.Timestamp != 0, "Should not return an empty store")
	assert.True(t, generator.Timestamp != 0, "Should not return an empty store")
	assert.NotEmpty(t, generator.Node, "Should not return an empty store")

	assert.True(t, generator.Timestamp < storageStamp, "Increment sequence when old timestamp newer than new")
	assert.Equal(t, Sequence(4), generator.Sequence, "Successfull read should have incremented sequence")

	// A new time that is older than stored time should cause the sequence to increment
	now, node := registerTestGenerator(Now().Sub(time.Second), nodeBytes)
	storageStamp = registerSaver(now.Add(time.Second), node)

	generator.read()

	assert.NotEqual(t, 0, generator.Sequence, "Should return an empty store")
	assert.NotEmpty(t, generator.Node, "Should not return an empty store")

	// A new time that is older than stored time should cause the sequence to increment
	registerTestGenerator(Now().Sub(time.Second), nil)
	storageStamp = registerSaver(now.Add(time.Second), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})

	generator.read()

	assert.NotEmpty(t, generator.Store, "Should not return an empty store")
	assert.NotEqual(t, []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb}, generator.Node, "Should not return an empty store")

	registerDefaultGenerator()
}

func TestGeneratorRandom(t *testing.T) {
	registerTestGenerator(Now(), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})

	b := make([]byte, 6)
	n, err := generator.Random(b)

	assert.NoError(t, err, "There should No be an error", err)
	assert.NotEmpty(t, b, "There should be random data in the slice")
	assert.Equal(t, 6, n, "Amount read should be same as length")

	generator.Random = func(b []byte) (int, error) {
		for i := 0; i < len(b); i++ {
			b[i] = byte(i)
		}
		return len(b), nil
	}

	b = make([]byte, 6)
	n, err = generator.Random(b)
	assert.NoError(t, err, "There should No be an error", err)
	assert.NotEmpty(t, b, "There should be random data in the slice")
	assert.Equal(t, 6, n, "Amount read should be same as length")

	generator.Random = func(b []byte) (int, error) {
		return 0, errors.New("EOF")
	}

	generator.HandleError = func(error) bool {
		return false
	}

	b = make([]byte, 6)
	c := []byte{}
	c = append(c, b...)

	n, err = generator.Random(b)
	assert.Error(t, err, "There should be an error", err)
	assert.Equal(t, 0, n, "Amount read should be same as length")
	assert.Equal(t, c, b, "Slice should be empty")

	id := NewV4()
	assert.Nil(t, id, "There should be no id")
	assert.Error(t, generator.err, "There should be an error [%s]", err)

	generator.HandleError = func(error) bool {
		return true
	}

	assert.Nil(t, NewV4(), "NewV4 should be nil")
	assert.Error(t, generator.err, "There should be an error [%s]", err)

	generator.HandleError = runHandleError

	assert.Panics(t, didNewV4Panic, "NewV4 should panic when invalid")
	assert.Error(t, generator.err, "There should be an error [%s]", err)

	generator.HandleError = func(error) bool {
		generator.Random = func([]byte) (int, error) {
			return 1, nil
		}
		return true
	}

	id = NewV4()
	assert.NotNil(t, id, "Should get an id")
	assert.NoError(t, generator.err, "There should be no error [%s]", err)

	registerDefaultGenerator()
}

func didNewV4Panic() {
	NewV4()
}

func TestGeneratorSave(t *testing.T) {
	registerTestGenerator(Now(), []byte{0xdd, 0xee, 0xff, 0xaa, 0xbb})
	generator.Do(generator.init)
	generator.read()
	generator.save()
	registerDefaultGenerator()
}

func TestGetHardwareAddress(t *testing.T) {
	a := findFirstHardwareAddress()
	assert.NotEmpty(t, a, "There should be a node id")
}

func registerTestGenerator(pNow Timestamp, pId Node) (Timestamp, Node) {
	generator = NewGenerator(GeneratorConfig{
		nil,
		func() Timestamp {
			return pNow
		}, 0,
		func() Node {
			return pId
		}, rand.Read,
		nil})
	return pNow, pId
}

func registerDefaultGenerator() {
	generator = NewGenerator(GeneratorConfig{})
}
