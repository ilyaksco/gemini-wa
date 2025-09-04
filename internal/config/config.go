package config

import (
	"log"
	"os"
	"strings"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKeys []string
	KnowledgeEnabled bool
	KnowledgeFile    string
	StoreLatitude    float64
	StoreLongitude   float64
	MenuImagePath    string
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

	knowledgeEnabled := os.Getenv("KNOWLEDGE_ENABLED") == "true"
	knowledgeFile := os.Getenv("KNOWLEDGE_FILE")

	lat, _ := strconv.ParseFloat(os.Getenv("STORE_LATITUDE"), 64)
	lon, _ := strconv.ParseFloat(os.Getenv("STORE_LONGITUDE"), 64)
	menuPath := os.Getenv("MENU_IMAGE_PATH")

	return &Config{
		GeminiAPIKeys: apiKeys,
		KnowledgeEnabled: knowledgeEnabled,
		KnowledgeFile:    knowledgeFile,
		StoreLatitude:    lat,
		StoreLongitude:   lon,
		MenuImagePath:    menuPath,
	}
}