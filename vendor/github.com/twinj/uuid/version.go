package uuid

// Version represents the type of UUID.
type Version uint8

const (
	VariantNCS       uint8 = 0x00
	VariantRFC4122   uint8 = 0x80 // or and A0 if masked with 1F
	VariantMicrosoft uint8 = 0xC0
	VariantFuture    uint8 = 0xE0
)

const (
	Unknown Version = iota // Unknown
	One                    // Time based
	Two                    // DCE security via POSIX UIDs
	Three                  // Namespace hash uses MD5
	Four                   // Crypto random
	Five                   // Namespace hash uses SHA-1
)

const (
	// 3f used by RFC4122 although 1f works for all
	variantSet = 0x3f

	// rather than using 0xc0 we use 0xe0 to retrieve the variant
	// The result is the same for all other variants
	// 0x80 and 0xa0 are used to identify RFC4122 compliance
	variantGet = 0xe0
)

// String returns English description of version.
func (o Version) String() string {
	switch o {
	case One:
		return "Version 1: Based on a 60 Bit Timestamp"
	case Two:
		return "Version 2: Based on DCE security domain and 60 bit timestamp"
	case Three:
		return "Version 3: Namespace UUID and unique names hashed by MD5"
	case Four:
		return "Version 4: Crypto-random"
	case Five:
		return "Version 5: Namespace UUID and unique names hashed by SHA-1"
	default:
		return "Unknown: Not supported"
	}
}

func resolveVersion(pVersion uint8) Version {
	switch Version(pVersion) {
	case One, Two, Three, Four, Five:
		return Version(pVersion)
	default:
		return Unknown
	}
}

func variant(pVariant uint8) uint8 {
	switch pVariant & variantGet {
	case VariantRFC4122, 0xA0:
		return VariantRFC4122
	case VariantMicrosoft:
		return VariantMicrosoft
	case VariantFuture:
		return VariantFuture
	}
	return VariantNCS
}
