package main

import (
	"testing"
)

func TestSafeMap_SetAndGet(t *testing.T) {
	safeMap := NewSafeMap[string, int]()
	safeMap.Set("key1", 1)

	value, ok := safeMap.Get("key1")
	if !ok || value != 1 {
		t.Errorf("Expected value 1 for key 'key1', got %d", value)
	}
}

func TestSafeMap_SetAndDelete(t *testing.T) {
	safeMap := NewSafeMap[string, int]()
	safeMap.Set("key1", 1)
	safeMap.Delete("key1")

	_, ok := safeMap.Get("key1")
	if ok {
		t.Errorf("Expected key 'key1' to be deleted")
	}
}

func TestSafeMap_Has(t *testing.T) {
	safeMap := NewSafeMap[string, int]()
	safeMap.Set("key1", 1)

	if !safeMap.Has("key1") {
		t.Errorf("Expected key 'key1' to exist")
	}
}

func TestSafeMap_Keys(t *testing.T) {
	safeMap := NewSafeMap[string, int]()
	safeMap.Set("key1", 1)
	safeMap.Set("key2", 2)

	keys := safeMap.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestSafeMap_Range(t *testing.T) {
	safeMap := NewSafeMap[string, int]()
	safeMap.Set("key1", 1)
	safeMap.Set("key2", 2)

	var keys []string
	err := safeMap.Range(func(k string, v int) error {
		keys = append(keys, k)
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}
