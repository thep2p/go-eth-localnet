package testutils

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

// ChannelsMustCloseWithinTimeout is a test helper that fails the test if any of the given channels do not close prior to the given timeout.
func ChannelsMustCloseWithinTimeout(t *testing.T, timeout time.Duration, failureMsg string, channels ...<-chan interface{}) {
	wg := sync.WaitGroup{}
	wg.Add(len(channels))

	for _, ch := range channels {
		go func(ch <-chan interface{}) {
			<-ch
			wg.Done()
		}(ch)
	}

	RequireCallMustReturnWithinTimeout(t, wg.Wait, timeout, failureMsg)
}