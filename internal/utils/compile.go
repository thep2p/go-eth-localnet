package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CompileFromStr compiles a Solidity contract from source string to binary and ABI format.
// Returns the compiled contract binary, ABI string, and an error if compilation fails.
func CompileFromStr(contractSource string) (
	contractBin string,
	contractABI string,
	err error) {

	// A temporary contract name for the compilation output
	contractName := "TempContract"

	// Step 1: Write contract string to temp .sol file
	tmpDir, err := os.MkdirTemp("", "solc")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			panic(fmt.Errorf("could not remove temp dir %s: %w", tmpDir, rmErr))
		}
	}()

	solPath := filepath.Join(tmpDir, contractName+".sol")
	if err := os.WriteFile(solPath, []byte(contractSource), 0644); err != nil {
		return "", "", fmt.Errorf("write contract: %w", err)
	}

	// Step 2: Run solc
	solcCmd := exec.Command("solc", "--combined-json", "abi,bin", solPath)
	output, err := solcCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("solc failed: %w", err)
	}

	// Step 3: Parse solc output
	var combined struct {
		Contracts map[string]struct {
			ABI json.RawMessage `json:"abi"`
			Bin string          `json:"bin"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(output, &combined); err != nil {
		return "", "", fmt.Errorf("unmarshal solc output: %w", err)
	}

	// solc uses the provided file path as part of the map key which may include
	// an absolute path. Since only one contract is being compiled, return the
	// first entry.
	for _, c := range combined.Contracts {
		return c.Bin, string(c.ABI), nil
	}

	return "", "", fmt.Errorf("compiled contract not found")
}
