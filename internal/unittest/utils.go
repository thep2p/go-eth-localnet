package unittest

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// RequireCallMustReturnWithinTimeout is a test helper that invokes the given function and fails the test if the invocation
// does not return prior to the given timeout.
func RequireCallMustReturnWithinTimeout(
	t *testing.T,
	f func(),
	timeout time.Duration,
	failureMsg string) {
	done := make(chan interface{})

	go func() {
		f()

		close(done)
	}()

	ChannelMustCloseWithinTimeout(
		t,
		done,
		timeout,
		fmt.Sprintf("function did not return on time: %s", failureMsg),
	)
}

// ChannelMustCloseWithinTimeout is a test helper that fails the test if the channel does not close prior to the given timeout.
func ChannelMustCloseWithinTimeout(
	t *testing.T,
	c <-chan interface{},
	timeout time.Duration,
	failureMsg string) {
	select {
	case <-c:
		return
	case <-time.After(timeout):
		require.Fail(t, fmt.Sprintf("channel did not close on time: %s", failureMsg))
	}
}

// DefaultReadyDoneTimeout is the default timeout for Ready/Done channel checks.
// This matches the skipgraph-go unittest default.
const DefaultReadyDoneTimeout = 10 * time.Second

// ReadyDoneAware is implemented by components with Ready and Done lifecycle channels.
// This interface mirrors github.com/thep2p/skipgraph-go/modules.ReadyDoneAware but uses
// chan struct{} which is idiomatic Go for signal-only channels.
type ReadyDoneAware interface {
	Ready() <-chan struct{}
	Done() <-chan struct{}
}

// RequireReady waits for the component to become ready within DefaultReadyDoneTimeout.
// Fails the test if the Ready channel doesn't close in time.
func RequireReady(t *testing.T, component ReadyDoneAware) {
	t.Helper()
	select {
	case <-component.Ready():
		return
	case <-time.After(DefaultReadyDoneTimeout):
		require.Fail(t, "component did not become ready within timeout")
	}
}

// RequireDone waits for the component to become done within DefaultReadyDoneTimeout.
// Fails the test if the Done channel doesn't close in time.
func RequireDone(t *testing.T, component ReadyDoneAware) {
	t.Helper()
	select {
	case <-component.Done():
		return
	case <-time.After(DefaultReadyDoneTimeout):
		require.Fail(t, "component did not become done within timeout")
	}
}

// RequireAllReady waits for all components to become ready within DefaultReadyDoneTimeout.
// Fails the test if any Ready channel doesn't close in time.
func RequireAllReady(t *testing.T, components ...ReadyDoneAware) {
	t.Helper()
	for i, c := range components {
		select {
		case <-c.Ready():
			continue
		case <-time.After(DefaultReadyDoneTimeout):
			require.Fail(t, fmt.Sprintf("component %d did not become ready within timeout", i))
		}
	}
}

// RequireAllDone waits for all components to become done within DefaultReadyDoneTimeout.
// Fails the test if any Done channel doesn't close in time.
func RequireAllDone(t *testing.T, components ...ReadyDoneAware) {
	t.Helper()
	for i, c := range components {
		select {
		case <-c.Done():
			continue
		case <-time.After(DefaultReadyDoneTimeout):
			require.Fail(t, fmt.Sprintf("component %d did not become done within timeout", i))
		}
	}
}
