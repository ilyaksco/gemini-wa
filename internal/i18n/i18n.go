package i18n

import (
	"encoding/json"
	"log"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func NewBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	_, err := bundle.LoadMessageFile("locales/en.json")
	if err != nil {
		log.Fatalf("Failed to load English translation file: %v", err)
	}
	_, err = bundle.LoadMessageFile("locales/id.json")
	if err != nil {
		log.Fatalf("Failed to load Indonesian translation file: %v", err)
	}

	log.Println("i18n bundle loaded successfully")
	return bundle
}

func NewLocalizer(bundle *i18n.Bundle, lang string) *i18n.Localizer {
	return i18n.NewLocalizer(bundle, lang)
}