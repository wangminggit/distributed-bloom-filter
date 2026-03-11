package bloom

import (
	"errors"
	"hash"

	"github.com/twmb/murmur3"
)

// ErrInvalidData is returned when deserializing invalid Bloom filter data.
var ErrInvalidData = errors.New("invalid bloom filter data")

// getHashIndices computes k hash indices for an item using double hashing technique.
// Double hashing uses two hash functions h1 and h2, and computes:
// h(i) = (h1(item) + i * h2(item)) mod m
// This reduces the number of hash functions needed while maintaining good distribution.
func getHashIndices(item []byte, m, k int) []int {
	indices := make([]int, k)

	// First hash: MurmurHash3
	h1 := uint64(murmur3.Sum32(item))

	// Second hash: Simple hash based on item length and first byte
	h2 := uint64(1 + (h1 % uint64(m-1)))

	for i := 0; i < k; i++ {
		indices[i] = int((h1 + uint64(i)*h2) % uint64(m))
	}

	return indices
}

// HashProvider defines the interface for hash functions used in Bloom filters.
type HashProvider interface {
	// Hash returns the hash value for the given data
	Hash(data []byte) uint64
	// Reset resets the hash state
	Reset()
	// Write writes data to the hash
	Write(p []byte) (n int, err error)
	// Sum computes the hash of the written data
	Sum(b []byte) []byte
}

// MurmurHash3Provider implements HashProvider using MurmurHash3.
type MurmurHash3Provider struct {
	hash.Hash32
}

// NewMurmurHash3Provider creates a new MurmurHash3 hash provider.
func NewMurmurHash3Provider() *MurmurHash3Provider {
	return &MurmurHash3Provider{
		Hash32: murmur3.New32(),
	}
}

// Hash returns the 64-bit hash value for the given data.
func (m *MurmurHash3Provider) Hash(data []byte) uint64 {
	m.Reset()
	m.Write(data)
	return uint64(m.Sum32())
}

// DoubleHash computes two hash values for double hashing technique.
// Returns (h1, h2) where both are in range [0, m).
func DoubleHash(item []byte, m int) (int, int) {
	h1 := murmur3.Sum32(item)

	// For h2, we hash the item with a different seed
	// Using a simple transformation to get a different hash
	h2Bytes := make([]byte, len(item)+4)
	copy(h2Bytes, item)
	h2Bytes[len(item)] = 0xDE
	h2Bytes[len(item)+1] = 0xAD
	h2Bytes[len(item)+2] = 0xBE
	h2Bytes[len(item)+3] = 0xEF

	h2 := murmur3.Sum32(h2Bytes)

	// Ensure h2 is in valid range and coprime to m
	h2 = 1 + (h2 % uint32(m-1))

	return int(h1 % uint32(m)), int(h2)
}

// ComputeIndices computes all k indices using double hashing.
func ComputeIndices(item []byte, m, k int) []int {
	h1, h2 := DoubleHash(item, m)
	indices := make([]int, k)

	for i := 0; i < k; i++ {
		indices[i] = (h1 + i*h2) % m
	}

	return indices
}
