package testutils

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"sync"
	"testing"
	"time"
)

// PrivateKeyFixture generates a new random private key for use in tests.
// It fails the test immediately if key generation does not succeed.
func PrivateKeyFixture(t *testing.T) *ecdsa.PrivateKey {
	priv, err := crypto.GenerateKey()
	require.NoError(t, err, "failed to generate private key")
	return priv
}

func RequireHandlersClosedWithinTimeout(t *testing.T, handles []*model.Handle, timeout time.Duration) {
	wg := sync.WaitGroup{}
	for _, h := range handles {
		wg.Add(1)
		go func(handle *model.Handle) {
			defer wg.Done()
			err := handle.Close()
			require.NoError(t, err, "failed to close node handle")
		}(h)
	}
	RequireCallMustReturnWithinTimeout(t, wg.Wait, timeout, "node handles did not close on time")
}
