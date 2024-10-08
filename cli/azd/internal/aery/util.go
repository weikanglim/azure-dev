package aery

func commonPrefix(a, b string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	if len(a) < len(b) {
		return a
	}

	return b
}

const (
	c1 uint32 = 0x239b961b
	c2 uint32 = 0xab0e9789
	c3 uint32 = 0x561ccd1b
	c4 uint32 = 0x0bcaa747
	c5 uint32 = 0x85ebca6b
	c6 uint32 = 0xc2b2ae35
)

// MurmurHash64 is the 64-bit variant of the MurmurHash2 algorithm (MurmurHash64A).
//
// MurmurHash64 is a largely lifted implementation from Azure's implementation.
// The original implementation can be found in Microsoft.WindowsAzure.ResourceStack.Common.Algorithms, as part of
// Azure.Deployments.Expression C# NuGet package. Credits given to the original authors.
//
// This implementation is cross-platform. It is not fully tested on big-endian systems.
func MurmurHash64(data []byte, seed uint32) uint64 {
	length := len(data)
	h1, h2 := seed, seed
	index := 0

	for index+7 < length {
		k1 := uint32(data[index+0]) | uint32(data[index+1])<<8 | uint32(data[index+2])<<16 | uint32(data[index+3])<<24
		k2 := uint32(data[index+4]) | uint32(data[index+5])<<8 | uint32(data[index+6])<<16 | uint32(data[index+7])<<24

		k1 *= c1
		k1 = rotateLeft32(k1, 15)
		k1 *= c2
		h1 ^= k1

		h1 = rotateLeft32(h1, 19)
		h1 += h2
		h1 = h1*5 + c3

		k2 *= c2
		k2 = rotateLeft32(k2, 17)
		k2 *= c1
		h2 ^= k2

		h2 = rotateLeft32(h2, 13)
		h2 += h1
		h2 = h2*5 + c4

		index += 8
	}

	tail := length - index
	if tail > 0 {
		var k1 uint32
		if tail >= 4 {
			k1 = uint32(data[index+0]) | uint32(data[index+1])<<8 | uint32(data[index+2])<<16 | uint32(data[index+3])<<24
		} else if tail == 3 {
			k1 = uint32(data[index+0]) | uint32(data[index+1])<<8 | uint32(data[index+2])<<16
		} else if tail == 2 {
			k1 = uint32(data[index+0]) | uint32(data[index+1])<<8
		} else {
			k1 = uint32(data[index+0])
		}

		k1 *= c1
		k1 = rotateLeft32(k1, 15)
		k1 *= c2
		h1 ^= k1

		if tail > 4 {
			var k2 uint32
			switch tail {
			case 7:
				k2 = uint32(data[index+4]) | uint32(data[index+5])<<8 | uint32(data[index+6])<<16
			case 6:
				k2 = uint32(data[index+4]) | uint32(data[index+5])<<8
			default:
				k2 = uint32(data[index+4])
			}

			k2 *= c2
			k2 = rotateLeft32(k2, 17)
			k2 *= c1
			h2 ^= k2
		}
	}

	h1 ^= uint32(length)
	h2 ^= uint32(length)

	h1 += h2
	h2 += h1

	h1 ^= h1 >> 16
	h1 *= c5
	h1 ^= h1 >> 13
	h1 *= c6
	h1 ^= h1 >> 16

	h2 ^= h2 >> 16
	h2 *= c5
	h2 ^= h2 >> 13
	h2 *= c6
	h2 ^= h2 >> 16

	h1 += h2
	h2 += h1

	return uint64(h2)<<32 | uint64(h1)
}

func rotateLeft32(x uint32, r uint) uint32 {
	return (x << r) | (x >> (32 - r))
}
