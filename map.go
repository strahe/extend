package main

import (
	"sync"
)

type SafeMap[kT comparable, vT any] struct {
	mu sync.RWMutex
	m  map[kT]vT
}

func NewSafeMap[kT comparable, vT any]() *SafeMap[kT, vT] {
	return &SafeMap[kT, vT]{
		m: make(map[kT]vT),
	}
}

func (sm *SafeMap[kT, vT]) Set(key kT, value vT) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

func (sm *SafeMap[kT, vT]) Get(key kT) (vT, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

func (sm *SafeMap[kT, vT]) Delete(key kT) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}

func (sm *SafeMap[kT, vT]) Has(key kT) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.m[key]
	return ok
}

func (sm *SafeMap[kT, vT]) Keys() []kT {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]kT, 0, len(sm.m))
	for k := range sm.m {
		keys = append(keys, k)
	}
	return keys
}

func (sm *SafeMap[kT, vT]) Range(f func(k kT, v vT) error) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for k, v := range sm.m {
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}
