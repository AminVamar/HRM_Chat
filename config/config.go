package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	UploadDir   string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Файл .env не найден, используются переменные окружения системы")
	}

	port := getEnv("PORT", "8080")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "12345")
	dbName := getEnv("DB_NAME", "chat_bd")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")
	uploadDir := getEnv("UPLOAD_DIR", "./uploads")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			dbUser, dbPass, dbHost, dbPort, dbName, dbSSLMode)
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		UploadDir:   uploadDir,
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPass,
		DBName:      dbName,
		DBSSLMode:   dbSSLMode,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
