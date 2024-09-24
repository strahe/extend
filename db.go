package main

import (
	"fmt"

	"github.com/xo/dburl"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(dbURI string) (*gorm.DB, error) {
	u, err := dburl.Parse(dbURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	fmt.Println(u.Driver, dbURI)
	var dialector gorm.Dialector
	switch u.Driver {
	case "sqlite3":
		dialector = sqlite.Open(u.DSN)
	case "mysql":
		dialector = mysql.Open(u.DSN)
	case "postgres":
		dialector = postgres.Open(u.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", u.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
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
