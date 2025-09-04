package main

import (
	"context"
	"gemini-whatsapp-bot/internal/bot"
	"gemini-whatsapp-bot/internal/config"
	"gemini-whatsapp-bot/internal/db"
	"gemini-whatsapp-bot/internal/i18n"
	"gemini-whatsapp-bot/internal/knowledge"
	geminiClient "gemini-whatsapp-bot/pkg/gemini"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	log.Println("Starting bot...")

	cfg := config.Load()
	db := db.New("bot_store.db")
	db.InitSchema()
	bundle := i18n.NewBundle()
	gemini := geminiClient.New(cfg.GeminiAPIKeys)
	knowledge := knowledge.Load(cfg.KnowledgeFile)


	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:bot_device.db?_foreign_keys=on", dbLog)
	if err != nil {
		log.Fatalf("Failed to create SQL store: %v", err)
	}
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatalf("Failed to get first device: %v", err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	handler := &bot.BotHandler{
		Client: client,
		DB:     db,
		Bundle: bundle,
		Gemini: gemini,
		Knowledge:        knowledge,
		KnowledgeEnabled: cfg.KnowledgeEnabled,
		StoreLatitude:    cfg.StoreLatitude,
		StoreLongitude:   cfg.StoreLongitude,
		MenuImagePath:    cfg.MenuImagePath,
	}
	client.AddEventHandler(handler.EventHandler)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				log.Println("QR code:", evt.Code)
			} else {
				log.Printf("Login event: %s", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		log.Println("Successfully connected to WhatsApp")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
	log.Println("Bot shut down gracefully")
}