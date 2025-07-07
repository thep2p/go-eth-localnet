package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileFromStr_Success(t *testing.T) {
	bin, abi, err := CompileFromStr("./internal/utils/contracts/TestContract1.sol")
	require.NoError(t, err)
	require.NotEmpty(t, bin)
	require.NotEmpty(t, abi)

	expectedABI := "[{\"inputs\":[],\"name\":\"f\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"}]"
	// expectedBin
	// := "6080604052348015600e575f5ffd5b5060af80601a5f395ff3fe6080604052348015600e575f5ffd5b50600436106026575f3560e01c806326121ff014602a575b5f5ffd5b60306044565b604051603b91906062565b60405180910390f35b5f602a905090565b5f819050919050565b605c81604c565b82525050565b5f60208201905060735f8301846055565b9291505056fea26469706673582212209a7acca45b99a3343ef11aafa228d4087fceae4984337049f063d5362e51a0ed64736f6c634300081e0033"

	require.Equal(t, expectedABI, abi)
	// require.Equal(t, expectedBin, bin)
}

// TestCompileFromStr_CommandFailure ensures an error is returned when solc fails.
func TestCompileFromStr_CommandFailure(t *testing.T) {

	contractSrc := "pragma solidity ^0.8.0;"

	tmpDir := t.TempDir()
	solcPath := filepath.Join(tmpDir, "solc")
	require.NoError(t, os.WriteFile(solcPath, []byte("#!/bin/sh\nexit 1"), 0755))

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+":"+origPath)

	bin, abi, err := CompileFromStr(contractSrc)
	require.Error(t, err)
	require.Empty(t, bin)
	require.Empty(t, abi)
}
