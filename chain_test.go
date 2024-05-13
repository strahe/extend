package main

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
)

func TestTimestampToEpoch(t *testing.T) {
	// Set up the genesis time
	// calibnet 2022-11-02 02:13:00 UTC+8
	_genesisTime := time.Date(2022, 11, 1, 18, 13, 0, 0, time.UTC)
	SetupGenesisTime(_genesisTime)

	t.Run("TimestampAfterGenesis", func(t *testing.T) {
		// Arrange
		timestamp := _genesisTime.Add(time.Hour) // 1 hour after genesis

		// Act
		epoch := TimestampToEpoch(timestamp)

		// Assert
		assert.Equal(t, abi.ChainEpoch(120), epoch) // 1 hour is 120 epochs
	})

	t.Run("TimestampAtGenesis", func(t *testing.T) {
		// Arrange
		timestamp := _genesisTime // At genesis

		// Act
		epoch := TimestampToEpoch(timestamp)

		// Assert
		assert.Equal(t, abi.ChainEpoch(0), epoch) // At genesis is epoch 0
	})
}

func TestSetupGenesisTime(t *testing.T) {
	t.Run("SetupGenesisTime", func(t *testing.T) {
		// Arrange
		expectedGenesisTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		// Act
		SetupGenesisTime(expectedGenesisTime)

		// Assert
		assert.Equal(t, expectedGenesisTime, genesisTime)
	})
}
