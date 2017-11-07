package uuid

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersion_String(t *testing.T) {
	for _, v := range []Version{
		One, Two, Three, Four, Five, Unknown,
	} {
		assert.NotEmpty(t, v.String(), "Expected a value")
	}
}

// Used to determine that a byte result from getting the variant is with the
// correct constraints and bounded values.
func tVariantConstraint(v byte, b byte, o UUID, t *testing.T) {
	switch v {
	case VariantNCS:
		switch b {
		case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
			break
		default:
			t.Errorf("%X most high bits do not resolve to 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07", b)
		}
	case VariantRFC4122:
		switch b {
		case 0x08, 0x09, 0x0A, 0x0B:
			break
		default:
			t.Errorf("%X most high bits do not resolve to 0x08, 0x09, 0x0A, 0x0B", b)
		}
	case VariantMicrosoft:
		switch b {
		case 0x0C, 0x0D:
			break
		default:
			t.Errorf("%X most high bits do not resolve to 0x0C, 0x0D", b)
		}
	case VariantFuture:
		switch b {
		case 0x0E, 0x0F:
			break
		default:
			t.Errorf("%X most high bits do not resolve to 0x0E, 0x0F", b)
		}
	}
}
