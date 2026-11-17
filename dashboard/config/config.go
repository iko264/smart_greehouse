package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Load membaca file .env lalu menyamakan zona waktu seluruh aplikasi.
func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env tidak ditemukan, memakai environment sistem")
	}

	if loc, err := time.LoadLocation(Env("APP_TIMEZONE", "Asia/Jakarta")); err == nil {
		time.Local = loc
	}
}

// Env mengambil nilai environment variable, jika kosong dipakai nilai fallback.
func Env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

// EnvInt sama seperti Env, tapi hasilnya angka.
func EnvInt(key string, fallback int) int {
	n, err := strconv.Atoi(Env(key, ""))
	if err != nil {
		return fallback
	}
	return n
}
