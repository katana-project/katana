package sync

import (
	"sync"
	"sync/atomic"
)

// KMutex hands out sync.Locker instances for locking by a key.
// Can be used as an alternative to golang.org/x/sync/singleflight.Group.
type KMutex struct {
	mu sync.Mutex
	mx map[string]*refCtMutex
}

// refCtMutex is a reference counted mutex.
type refCtMutex struct {
	sync.Mutex

	refCount int32
}

// Make returns a mutex for the specified key with its reference counter incremented by one, created if not present.
func (km *KMutex) Make(key string) sync.Locker {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.mx == nil {
		km.mx = make(map[string]*refCtMutex, 1)
	} else if mu, ok := km.mx[key]; ok {
		atomic.AddInt32(&mu.refCount, 1)
		return mu
	}

	mu := &refCtMutex{refCount: 1}
	km.mx[key] = mu
	return mu
}

// Release decrements the reference counter for the mutex with the supplied key, removing it entirely if it reaches zero.
func (km *KMutex) Release(key string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if mu, ok := km.mx[key]; ok {
		if atomic.AddInt32(&mu.refCount, -1) <= 0 {
			delete(km.mx, key)
		}
	}
}

// Do acquires a lock, runs the action and releases it.
func (km *KMutex) Do(key string, action func() (interface{}, error)) (interface{}, error) {
	mu := km.Make(key)
	mu.Lock()

	res, err := action()

	mu.Unlock()
	km.Release(key)

	return res, err
}
