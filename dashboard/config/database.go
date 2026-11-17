package config

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB dipakai di seluruh aplikasi.
var DB *gorm.DB

func ConnectDatabase() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		Env("DB_USERNAME", "root"),
		Env("DB_PASSWORD", ""),
		Env("DB_HOST", "127.0.0.1"),
		Env("DB_PORT", "3306"),
		Env("DB_DATABASE", "smart_greenhouse"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		// Hanya tampilkan query yang lambat atau error, supaya log tidak penuh insert sensor.
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}

	DB = db
	log.Println("Database terhubung")
}
