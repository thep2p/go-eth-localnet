// Package consensus provides abstractions for Consensus Layer client implementations.
//
// This package defines interfaces and types that enable support for multiple
// CL clients (Prysm, Lighthouse, Nimbus, etc.) without duplicating code.
package consensus

import (
	"context"
)

// Client represents a Consensus Layer client instance.
//
// Client implementations manage the lifecycle of CL clients and provide
// access to their operational state and metrics.
type Client interface {
	// Start begins the CL client's operation.
	// The provided context controls the client's lifetime.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the CL client.
	// After Stop returns, Wait should be called to ensure cleanup completes.
	Stop() error

	// Wait blocks until the client is fully stopped.
	// Should be called after Stop to ensure all resources are released.
	Wait() error

	// Ready returns true when the client is synced and operational.
	// A client is ready when it can participate in consensus.
	Ready() bool

	// BeaconEndpoint returns the Beacon API endpoint URL.
	// This endpoint can be used for querying beacon chain state.
	BeaconEndpoint() string

	// ValidatorKeys returns the validator public keys managed by this client.
	// Returns an empty slice if no validators are configured.
	ValidatorKeys() []string

	// Metrics returns client metrics (slots, peers, etc.).
	// Returns an error if metrics cannot be retrieved.
	Metrics() (*Metrics, error)
}

// Metrics contains CL client operational metrics.
//
// These metrics provide visibility into the consensus client's state
// and can be used for monitoring and debugging.
type Metrics struct {
	// CurrentSlot is the current slot number in the beacon chain.
	CurrentSlot uint64

	// HeadSlot is the slot number of the current head block.
	HeadSlot uint64

	// FinalizedSlot is the slot number of the most recent finalized block.
	FinalizedSlot uint64

	// PeerCount is the number of connected peers.
	PeerCount int

	// IsSyncing indicates whether the client is currently syncing.
	IsSyncing bool

	// ValidatorCount is the number of active validators.
	ValidatorCount int
}
