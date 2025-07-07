package utils

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// CompileFromStr compiles a Solidity contract from source string to binary and ABI format.
// Returns the compiled contract binary, ABI string, and an error if compilation fails.
func CompileFromStr(solPath string) (
	contractBin string,
	contractABI string,
	err error) {

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
