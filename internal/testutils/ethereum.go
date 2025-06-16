package testutils

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func PrivateKeyFixture(t *testing.T) *ecdsa.PrivateKey {
	priv, err := crypto.GenerateKey()
	require.NoError(t, err, "failed to generate private key")
	return priv
}
