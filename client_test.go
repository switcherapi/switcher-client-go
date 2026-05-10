package client

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientGetSwitcher(t *testing.T) {
	t.Run("should return cached instance for the same key", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
		})

		switcher1 := GetSwitcher("switcher1")
		switcher2 := GetSwitcher("switcher1")
		switcher3 := GetSwitcher("")

		assert.Same(t, switcher1, switcher2, "expected the same instance for the same key")
		assert.NotSame(t, switcher1, switcher3, "expected different instances for different keys")
	})

	t.Run("should return the cached instance after concurrent insert", func(t *testing.T) {
		client := NewClient(Context{Domain: "My Domain"})

		blockFirst := make(chan struct{})
		firstMissReached := make(chan struct{})
		var hookCalls atomic.Int32

		getSwitcherAfterReadMissHook = func() {
			if hookCalls.Add(1) == 1 {
				close(firstMissReached)
				<-blockFirst
			}
		}
		defer func() {
			getSwitcherAfterReadMissHook = nil
		}()

		var wg sync.WaitGroup
		var first *Switcher

		wg.Go(func() {
			first = client.GetSwitcher("switcher1")
		})

		<-firstMissReached

		second := client.GetSwitcher("switcher1")
		close(blockFirst)
		wg.Wait()

		assert.Same(t, first, second, "expected the same instance after concurrent insert")
		assert.Len(t, client.switchers, 1, "expected only one instance in the cache")
	})
}

func TestDefaultClient(t *testing.T) {
	t.Run("should initialize the singleton when unset", func(t *testing.T) {
		savedClient := globalClient.Load()
		savedHook := defaultClientBeforeCompareAndSwapHook
		globalClient.Store(nil)
		defaultClientBeforeCompareAndSwapHook = nil
		defer func() {
			globalClient.Store(savedClient)
			defaultClientBeforeCompareAndSwapHook = savedHook
		}()

		client := defaultClient()

		assert.NotNil(t, client)
		assert.Same(t, client, globalClient.Load(), "expected the singleton to be initialized")
	})

	t.Run("should return the loaded client after concurrent initialization", func(t *testing.T) {
		savedClient := globalClient.Load()
		savedHook := defaultClientBeforeCompareAndSwapHook
		globalClient.Store(nil)
		defer func() {
			globalClient.Store(savedClient)
			defaultClientBeforeCompareAndSwapHook = savedHook
		}()

		blockFirst := make(chan struct{})
		firstCreated := make(chan struct{})
		var hookCalls atomic.Int32

		defaultClientBeforeCompareAndSwapHook = func() {
			if hookCalls.Add(1) == 1 {
				close(firstCreated)
				<-blockFirst
			}
		}

		var got *Client
		var wg sync.WaitGroup
		wg.Go(func() {
			got = defaultClient()
		})

		<-firstCreated

		expected := NewClient(Context{Domain: "My Domain"})
		globalClient.Store(expected)

		close(blockFirst)
		wg.Wait()

		assert.Same(t, expected, got, "expected the loaded client after concurrent initialization")
		assert.Same(t, expected, globalClient.Load(), "expected the global client to be the same as the loaded client")
	})
}
