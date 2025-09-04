package knowledge

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Knowledge struct {
	Content string
}

type yamlData struct {
	Knowledge string `yaml:"knowledge"`
}

func Load(filePath string) *Knowledge {
	if filePath == "" {
		log.Println("Knowledge file path is not provided, skipping.")
		return &Knowledge{Content: ""}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Could not read knowledge file at %s: %v", filePath, err)
		return &Knowledge{Content: ""}
	}

	var parsed yamlData
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		log.Printf("Could not parse knowledge YAML file: %v", err)
		return &Knowledge{Content: ""}
	}

	log.Println("Knowledge base loaded successfully.")
	return &Knowledge{Content: parsed.Knowledge}
}