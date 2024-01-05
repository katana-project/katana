package sync

import (
	"fmt"
	"sync"
	"testing"
)

const (
	key           = "testKey"
	numGoroutines = 10
	numIterations = 100
)

func TestKMutex(t *testing.T) {
	var km KMutex
	t.Run("Make", func(t *testing.T) {
		mu := km.Make(key)
		mu.Lock()
		mu.Unlock()

		if _, ok := km.mx[key]; !ok {
			t.Errorf("key not found")
		}
	})

	t.Run("Release", func(t *testing.T) {
		km.Release(key)

		if _, ok := km.mx[key]; ok {
			t.Errorf("key still found")
		}
	})
}

func TestKMutexConcurrentLocking(t *testing.T) {
	var (
		km KMutex
		wg sync.WaitGroup
	)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				mu := km.Make(key)
				mu.Lock()

				fmt.Printf("acquired lock for goroutine %d, iteration %d\n", index, j)

				mu.Unlock()
				km.Release(key)
			}
		}(i)
	}

	wg.Wait()
	if _, ok := km.mx[key]; ok {
		t.Errorf("key still found")
	}
}

func TestKMutexReferenceCounting(t *testing.T) {
	var (
		km KMutex
		mu = km.Make(key)
	)

	if mu != km.Make(key) {
		t.Error("expected the same mutex for the same key")
	}

	km.Release(key)

	if _, ok := km.mx[key]; !ok {
		t.Error("expected mutex to remain after releasing for the first time")
	}

	mu.Lock()
	mu.Unlock()

	km.Release(key)

	if _, ok := km.mx[key]; ok {
		t.Error("expected mutex to be deleted after releasing for the second time")
	}
}
