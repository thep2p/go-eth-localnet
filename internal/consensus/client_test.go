package consensus_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/unittest/mocks"
)

// TestClientBeaconEndpoint verifies that BeaconEndpoint returns the configured endpoint.
func TestClientBeaconEndpoint(t *testing.T) {
	mockClient := mocks.NewMockClient(t)

	// Set expectation
	mockClient.EXPECT().
		BeaconEndpoint().
		Return("http://127.0.0.1:4000").
		Once()

	// Test
	endpoint := mockClient.BeaconEndpoint()
	require.Equal(t, "http://127.0.0.1:4000", endpoint,
		"beacon endpoint should match expected value")
}

// TestClientValidatorKeys verifies that ValidatorKeys returns the configured keys.
func TestClientValidatorKeys(t *testing.T) {
	mockClient := mocks.NewMockClient(t)
	expectedKeys := []string{"key1", "key2", "key3"}

	// Set expectation
	mockClient.EXPECT().
		ValidatorKeys().
		Return(expectedKeys).
		Once()

	// Test
	keys := mockClient.ValidatorKeys()
	require.Equal(t, expectedKeys, keys,
		"validator keys should match expected value")
}

// TestClientMetrics verifies that Metrics returns metrics successfully.
func TestClientMetrics(t *testing.T) {
	mockClient := mocks.NewMockClient(t)
	expectedMetrics := &consensus.Metrics{
		CurrentSlot:    100,
		HeadSlot:       100,
		FinalizedSlot:  95,
		PeerCount:      10,
		IsSyncing:      false,
		ValidatorCount: 3,
	}

	// Set expectation
	mockClient.EXPECT().
		Metrics().
		Return(expectedMetrics, nil).
		Once()

	// Test
	metrics, err := mockClient.Metrics()
	require.NoError(t, err, "metrics should be retrieved successfully")
	require.Equal(t, expectedMetrics, metrics,
		"metrics should match expected value")
}

// TestClientMetricsError verifies that Metrics can return errors.
func TestClientMetricsError(t *testing.T) {
	mockClient := mocks.NewMockClient(t)

	// Set expectation for error case
	testErr := &metricsError{msg: "client not ready"}
	mockClient.EXPECT().
		Metrics().
		Return(nil, testErr).
		Once()

	// Test
	metrics, err := mockClient.Metrics()
	require.Error(t, err, "metrics should return error")
	require.Nil(t, metrics, "metrics should be nil on error")
	require.Contains(t, err.Error(), "not ready",
		"error message should indicate client is not ready")
}

// metricsError is a test error type for metrics operations.
type metricsError struct {
	msg string
}

func (e *metricsError) Error() string {
	return e.msg
}

// TestClientMultipleMetricsCalls verifies multiple calls to Metrics.
func TestClientMultipleMetricsCalls(t *testing.T) {
	mockClient := mocks.NewMockClient(t)

	// First call returns metrics with slot 100
	metrics1 := &consensus.Metrics{CurrentSlot: 100}
	mockClient.EXPECT().
		Metrics().
		Return(metrics1, nil).
		Once()

	// Second call returns metrics with slot 101
	metrics2 := &consensus.Metrics{CurrentSlot: 101}
	mockClient.EXPECT().
		Metrics().
		Return(metrics2, nil).
		Once()

	// Test first call
	result1, err := mockClient.Metrics()
	require.NoError(t, err)
	require.Equal(t, uint64(100), result1.CurrentSlot)

	// Test second call
	result2, err := mockClient.Metrics()
	require.NoError(t, err)
	require.Equal(t, uint64(101), result2.CurrentSlot)
}
