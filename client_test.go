package client

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestClientGetSwitcherFromCache(t *testing.T) {
	BuildContext(Context{
		Domain: "My Domain",
	})

	switcher1 := GetSwitcher("switcher1")
	switcher2 := GetSwitcher("switcher1")
	switcher3 := GetSwitcher("")

	if switcher1 != switcher2 {
		t.Fatalf("expected cached switcher instance for the same key")
	}

	if switcher1 == switcher3 {
		t.Fatalf("expected empty-key switcher to be a distinct instance")
	}
}

func TestClientGetSwitcherReturnsCachedInstanceAfterConcurrentInsert(t *testing.T) {
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		first = client.GetSwitcher("switcher1")
	}()

	<-firstMissReached

	second := client.GetSwitcher("switcher1")
	close(blockFirst)
	wg.Wait()

	if first != second {
		t.Fatalf("expected concurrent callers to receive the same cached switcher instance")
	}

	if len(client.switchers) != 1 {
		t.Fatalf("expected exactly one cached switcher, got %d", len(client.switchers))
	}
}

func TestDefaultClientInitializesSingletonWhenUnset(t *testing.T) {
	savedClient := globalClient.Load()
	savedHook := defaultClientBeforeCompareAndSwapHook
	globalClient.Store(nil)
	defaultClientBeforeCompareAndSwapHook = nil
	defer func() {
		globalClient.Store(savedClient)
		defaultClientBeforeCompareAndSwapHook = savedHook
	}()

	client := defaultClient()

	if client == nil {
		t.Fatalf("expected default client to be initialized")
	}

	if globalClient.Load() != client {
		t.Fatalf("expected initialized client to be stored globally")
	}
}

func TestDefaultClientReturnsLoadedClientAfterConcurrentInitialization(t *testing.T) {
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		got = defaultClient()
	}()

	<-firstCreated

	expected := NewClient(Context{Domain: "My Domain"})
	globalClient.Store(expected)

	close(blockFirst)
	wg.Wait()

	if got != expected {
		t.Fatalf("expected fallback path to return the concurrently stored client")
	}

	if globalClient.Load() != expected {
		t.Fatalf("expected concurrently stored client to remain the global singleton")
	}
}
