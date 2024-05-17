package main

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	// Migrate the schema
	if err := db.AutoMigrate(&Request{}, &Message{}, &Extension2{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}
	return db, nil
}
