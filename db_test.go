package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDB_SuccessfulConnection(t *testing.T) {
	dbPath := "_test.db"
	_, err := NewDB(dbPath)
	assert.NoError(t, err)

	// Add this defer function to remove the database file after test
	defer func() {
		err := os.Remove(dbPath)
		if err != nil {
			t.Fatal(err)
		}
	}()
}

func TestNewDB_FailedConnection(t *testing.T) {
	dbPath := "invalid/path/to/db"
	_, err := NewDB(dbPath)
	assert.Error(t, err)
}
