package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileFromStr_Success(t *testing.T) {
	contractSrc := `
		// SPDX-License-Identifier: MIT
		pragma solidity ^0.8.0;
		contract TempContract {
			function f() public pure returns (uint) {
				return 42;
			}
		}`

	bin, abi, err := CompileFromStr(contractSrc)
	require.NoError(t, err)
	require.NotEmpty(t, bin)
	require.NotEmpty(t, abi)
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
