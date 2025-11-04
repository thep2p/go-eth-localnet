package unittest

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
)

// PrivateKeyFixture generates a new random private key for use in tests.
// It fails the test immediately if key generation does not succeed.
func PrivateKeyFixture(t *testing.T) *ecdsa.PrivateKey {
	priv, err := crypto.GenerateKey()
	require.NoError(t, err, "failed to generate private key")
	return priv
}

// GetBalance retrieves the balance of the given address from the Ethereum node.
func GetBalance(
	t *testing.T,
	ctx context.Context,
	client *rpc.Client,
	address common.Address,
) *big.Int {
	var balHex string
	require.NoError(t, client.CallContext(ctx, &balHex, "eth_getBalance", address.Hex(), "latest"))
	return HexToBigInt(t, balHex)
}

func HexToBigInt(t *testing.T, hexStr string) *big.Int {
	bi, ok := new(big.Int).SetString(strings.TrimPrefix(hexStr, "0x"), 16)
	require.True(t, ok, "failed to convert hex to big.Int: %s", hexStr)
	return bi
}
