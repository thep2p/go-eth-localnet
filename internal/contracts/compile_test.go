package contracts_test

import (
	"github.com/thep2p/go-eth-localnet/internal/contracts"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompileFromStr_Success tests the successful compilation of a Solidity contract.
func TestCompileFromStr_Success(t *testing.T) {
	bin, abi, err := contracts.GenerateAbiAndBin("SimpleTestContract.sol")
	require.NoError(t, err)
	require.NotEmpty(t, bin)
	require.NotEmpty(t, abi)

	expectedABI := "[{\"inputs\":[],\"name\":\"f\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"}]"
	expectedBin := "6080604052348015600e575f5ffd5b50608680601a5f395ff3fe6080604052348015600e575f5ffd5b50600436106026575f3560e01c806326121ff014602a575b5f5ffd5b60306044565b604051603b91906062565b60405180910390f35b5f602a905090565b5f819050919050565b605c81604c565b82525050565b5f60208201905060735f8301846055565b9291505056fea164736f6c634300081e000a"

	require.Equal(t, expectedABI, abi)
	require.Equal(t, expectedBin, bin)
}

// TestGenerateAbiAndBin_FileNotFound tests the case where the specified Solidity file does not exist.
// It should return an error indicating the file was not found.
func TestGenerateAbiAndBin_FileNotFound(t *testing.T) {
	bin, abi, err := contracts.GenerateAbiAndBin("NonExistentContract.sol")
	require.Error(t, err)
	require.Contains(t, err.Error(), "file not found")
	require.Empty(t, bin)
	require.Empty(t, abi)
}

// TestGenerateAbiAndBin_EmptyContracts tests the case where the Solidity file has an empty contract.
func TestGenerateAbiAndBin_EmptyContracts(t *testing.T) {
	bin, abi, err := contracts.GenerateAbiAndBin("EmptyContract.sol")
	require.NoError(t, err)
	require.Equal(t, "6080604052348015600e575f5ffd5b50601580601a5f395ff3fe60806040525f5ffdfea164736f6c634300081e000a", bin)
	require.Equal(t, "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]", abi)
}
