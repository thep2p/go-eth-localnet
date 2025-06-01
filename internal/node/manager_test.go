package node_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"testing"
)

func TestStartMultipleNodes(t *testing.T) {
	t.Parallel()
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.RandomPort(t), testutils.RandomPort(t)+1000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx, 3)
	require.NoError(t, err)
	require.Len(t, manager.Handles(), 3)
}
