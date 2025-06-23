package testutils

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

// PrivateKeyFixture generates a new random private key for use in tests.
// It fails the test immediately if key generation does not succeed.
func PrivateKeyFixture(t *testing.T) *ecdsa.PrivateKey {
	priv, err := crypto.GenerateKey()
	require.NoError(t, err, "failed to generate private key")
	return priv
}
