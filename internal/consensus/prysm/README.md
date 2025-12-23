# Prysm Package

Package prysm provides utilities for Prysm beacon chain genesis state generation.

## Overview

The prysm package currently implements genesis state generation for Prysm consensus layer nodes. This allows creating deterministic genesis states with configured validator keys and withdrawal addresses for local Ethereum network development and testing.

## Current Features

### Genesis State Generation

Generate beacon chain genesis states programmatically using the Prysm v5 API:

```go
// Generate test validator keys
validatorKeys, err := prysm.GenerateValidatorKeys(4)
if err != nil {
    log.Fatal(err)
}

// Create unique withdrawal address for each validator
withdrawalAddrs := []common.Address{
    common.HexToAddress("0x1111111111111111111111111111111111111111"),
    common.HexToAddress("0x2222222222222222222222222222222222222222"),
    common.HexToAddress("0x3333333333333333333333333333333333333333"),
    common.HexToAddress("0x4444444444444444444444444444444444444444"),
}

// Create consensus config
// Genesis time should be 30 seconds in the past for immediate block production
cfg := consensus.Config{
    ChainID:             1337,
    GenesisTime:         time.Now().Add(-30 * time.Second),
    ValidatorKeys:       validatorKeys,
    WithdrawalAddresses: withdrawalAddrs,
    FeeRecipient:        withdrawalAddrs[0],
    // Other fields required by Config validation...
    DataDir:        "/tmp/prysm",
    BeaconPort:     4000,
    P2PPort:        9000,
    EngineEndpoint: "http://127.0.0.1:8551",
    JWTSecret:      jwtSecret,
}

// Generate SSZ-encoded genesis state
genesisState, err := prysm.GenerateGenesisState(cfg)
if err != nil {
    log.Fatal(err)
}

// Derive genesis root (32-byte hash tree root)
genesisRoot, err := prysm.DeriveGenesisRoot(genesisState)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Genesis root: %s\n", genesisRoot.Hex())
```

## Available Functions

### `GenerateValidatorKeys(count int) ([]bls.SecretKey, error)`

Generates deterministic BLS12-381 validator keys for testing. Keys are generated using Prysm's interop key generation, ensuring consistency across test runs.

**WARNING:** These keys are NOT production-safe. Only use for local development and testing.

**Example:**
```go
keys, err := prysm.GenerateValidatorKeys(64)
if err != nil {
    log.Fatal(err)
}
// keys[0], keys[1], ... are deterministic BLS secret keys
```

### `GenerateGenesisState(cfg consensus.Config) ([]byte, error)`

Creates a beacon chain genesis state from configuration. Returns SSZ-encoded genesis state containing all validators, balances, committees, and historical roots.

**Each validator requires:**
- BLS secret key (for signing)
- Withdrawal address (for rewards and stake withdrawals)
- 32 ETH stake (MaxEffectiveBalance)

**Returns:**
- SSZ-encoded beacon state (full state, not just the root)
- Error if configuration is invalid

**All errors are CRITICAL** and indicate genesis state generation cannot proceed.

### `DeriveGenesisRoot(genesisState []byte) (common.Hash, error)`

Calculates the 32-byte hash tree root from SSZ-encoded genesis state. This root is used as the network identifier in the consensus layer.

**Returns:**
- 32-byte genesis beacon state root
- Error if state is invalid or corrupted

**All errors are CRITICAL** and indicate the genesis state is invalid.

## Withdrawal Credentials

Each validator must have a withdrawal address configured. The package uses Type 0x01 withdrawal credentials (direct withdrawal to Ethereum address), which is the modern standard post-Shanghai upgrade.

Withdrawal credential structure:
```
[0x01][11 zero bytes][20-byte Ethereum address]
 ^^^^^               ^^^^^^^^^^^^^^^^^^^^^^^
 type                withdrawal address
```

- **Type 0x01** - Direct withdrawal to Ethereum address (modern standard)
- **Type 0x00** - BLS withdrawal credentials (legacy, pre-Shanghai)

## Genesis State Structure

The generated genesis state includes:

1. **Validator Registry** - All validators with public keys, withdrawal credentials, and balances
2. **Deposit Tree** - Merkle tree of all deposits (in eth1_data field)
3. **Genesis Time** - Network start time (Unix timestamp)
4. **Fork Information** - Genesis fork version from Prysm config
5. **Committee Configuration** - Validator committee assignments

## Testing

Comprehensive test coverage in `genesis_test.go`:

- `TestGenerateValidatorKeys` - Key generation and uniqueness
- `TestGenerateValidatorKeysDeterminism` - Deterministic key generation
- `TestGenerateGenesisState` - Genesis state generation
- `TestGenerateGenesisStateValidation` - Configuration validation
- `TestDeriveGenesisRoot` - Genesis root derivation
- `TestDeriveGenesisRootDeterminism` - Deterministic root calculation

All tests pass and verify working functionality.

## Implementation Status

**Currently Implemented (100% complete):**
- ✅ Genesis state generation
- ✅ Genesis root derivation
- ✅ Deterministic validator key generation
- ✅ Withdrawal address configuration
- ✅ Comprehensive test coverage

**Planned for Future Issues:**

The following features will be implemented in subsequent PRs when the functionality is complete:

- **Issue #45**: Prysm beacon node lifecycle management (initialization, startup, shutdown)
- **Issue #46**: Prysm validator client integration with BLS key management
- **Issue #48**: Beacon API health checks and readiness probes
- **Issue #49**: Prysm-Geth integration tests (full Engine API communication)

**Note:** These features will be implemented from 0% to 100% with zero placeholders or stubs, following the project's "Complete Implementation Over Placeholder Code" principle.

## Design Philosophy

This package follows the project's core principles:

1. **Complete Implementation** - Only fully working code is committed
2. **No Placeholders** - No stub methods, skipped tests, or TODOs in core functionality
3. **Vertical Slicing** - Each feature is complete and testable on its own
4. **Testability** - All code has comprehensive test coverage that verifies real behavior

Each function in this package:
- Has a complete, working implementation
- Includes full test coverage
- Is demonstrable and deployable
- Has zero TODOs or "not implemented" errors

## See Also

- `internal/consensus/config.go` - Configuration data structures
- `internal/node` - Geth execution layer node management
- [github.com/prysmaticlabs/prysm/v5](https://github.com/prysmaticlabs/prysm) - Prysm consensus client
