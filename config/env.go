package config

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func LoadDotEnv() {
	content, err := os.ReadFile(".env")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("No .env file found in current directory; using system environment")
			return
		}
		log.Printf("Could not read .env: %T; using system environment", err)
		return
	}

	env, err := godotenv.Unmarshal(strings.TrimPrefix(string(content), "\ufeff"))
	if err != nil {
		log.Printf("Could not parse .env: %T; using system environment", err)
		return
	}

	for key, value := range env {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			log.Printf("Could not set .env value for %s: %T", key, err)
		}
	}
}
