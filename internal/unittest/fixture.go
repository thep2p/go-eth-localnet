package unittest

import (
	"crypto/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// RandomAddress generates a random Ethereum address for testing.
//
// This function generates 20 cryptographically random bytes and converts
// them to a common.Address. It is useful for creating test fixtures where
// the specific address value doesn't matter.
//
// The function will fail the test if random byte generation fails.
func RandomAddress(t *testing.T) common.Address {
	t.Helper()

	b := make([]byte, 20)
	_, err := rand.Read(b)
	require.NoError(t, err, "failed to generate random bytes for address")

	return common.BytesToAddress(b)
}

// RandomAddresses generates n random Ethereum addresses for testing.
//
// This is a convenience function for generating multiple random addresses
// at once. Each address is independently generated with cryptographic randomness.
//
// The function will fail the test if random byte generation fails.
func RandomAddresses(t *testing.T, n int) []common.Address {
	t.Helper()

	addrs := make([]common.Address, n)
	for i := 0; i < n; i++ {
		addrs[i] = RandomAddress(t)
	}
	return addrs
}
