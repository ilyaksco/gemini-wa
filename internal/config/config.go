package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKeys []string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	apiKeysStr := os.Getenv("GEMINI_API_KEYS")
	if apiKeysStr == "" {
		log.Fatal("GEMINI_API_KEYS is not set in .env file or environment variables")
	}

	apiKeys := strings.Split(apiKeysStr, ",")
	if len(apiKeys) == 0 || apiKeys[0] == "" {
		log.Fatal("GEMINI_API_KEYS is empty or invalid")
	}

	log.Printf("Loaded %d Gemini API keys", len(apiKeys))

	return &Config{
		GeminiAPIKeys: apiKeys,
	}
}