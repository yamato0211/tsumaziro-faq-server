package config

import (
	"os"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	DBHost string
	DBName string
	DBUser string
	DBPass string
	DBPort string
	UseTLS string
}

type FirebaseConfig struct {
	FirebaseSecret string
}

func NewDBConfig() *DBConfig {
	godotenv.Load()

	cfg := &DBConfig{
		DBHost: os.Getenv("DB_HOST"),
		DBName: os.Getenv("DB_DATABASE"),
		DBUser: os.Getenv("DB_USERNAME"),
		DBPass: os.Getenv("DB_PASSWORD"),
		DBPort: os.Getenv("DB_PORT"),
		UseTLS: os.Getenv("DB_USE_TLS"),
	}
	return cfg
}

func NewFirebaseConfig() *FirebaseConfig {
	godotenv.Load()

	cfg := &FirebaseConfig{
		FirebaseSecret: os.Getenv("FIREBASE_SECRET"),
	}

	return cfg
}
