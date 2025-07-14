package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// GenerateAbiAndBin compiles a Solidity contract and returns its ABI and binary code or an error.
func GenerateAbiAndBin(solPath string) (
	contractBin string,
	contractABI string,
	err error) {

	// Step 1a: Check if solc is installed
	if _, err := exec.LookPath("solc"); err != nil {
		return "", "", fmt.Errorf("solc not found in PATH: %w", err)
	}

	// Step 1b: Check if the provided file exists
	if _, err := os.Stat(solPath); err != nil {
		return "", "", fmt.Errorf("file not found: %w", err)
	}

	// Step 2: Run solc
	solcCmd := exec.Command("solc", "--combined-json", "abi,bin", "--metadata-hash", "none", solPath)
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
