package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDB_SuccessfulConnection1(t *testing.T) {
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

func TestNewDB_SuccessfulConnection2(t *testing.T) {
	dbURI := "sqlite3://:memory:"
	db, err := NewDB(dbURI)
	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestNewDB_InvalidURI(t *testing.T) {
	dbURI := "invalid_uri"
	_, err := NewDB(dbURI)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse database URL")
}

func TestNewDB_UnsupportedDriver(t *testing.T) {
	dbURI := "sqlserver://user:pass@remote-host.com/dbname"
	_, err := NewDB(dbURI)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver")
}
