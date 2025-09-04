package bot

import (
	"context"
	"fmt"
	"gemini-whatsapp-bot/internal/db"
	"gemini-whatsapp-bot/internal/knowledge"
	geminiClient "gemini-whatsapp-bot/pkg/gemini"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type BotHandler struct {
	Client *whatsmeow.Client
	DB     *db.Database
	Bundle *goi18n.Bundle
	Gemini *geminiClient.Client
	Knowledge        *knowledge.Knowledge
	KnowledgeEnabled bool
	StoreLatitude    float64
	StoreLongitude   float64
	MenuImagePath    string
}

func (h *BotHandler) EventHandler(evt interface{}) {
	log.Printf("Received a new event of type: %T", evt)
	switch v := evt.(type) {
	case *events.Message:
		h.handleMessage(v)
	}
}

func (h *BotHandler) handleMessage(msg *events.Message) {
	senderJID := msg.Info.Sender.String()
	log.Printf("Processing message event from %s", senderJID)

	if msg.Info.IsFromMe {
		log.Println("Message is from me, ignoring")
		return
	}

	userLang := h.DB.GetUserLang(senderJID)
	localizer := goi18n.NewLocalizer(h.Bundle, userLang)
	chatJID := msg.Info.Chat

	if img := msg.Message.GetImageMessage(); img != nil {
		h.handleImageMessage(img, chatJID, senderJID, localizer)
		return
	}

	var text string
	if msg.Message.GetConversation() != "" {
		text = msg.Message.GetConversation()
	} else if extMsg := msg.Message.GetExtendedTextMessage(); extMsg != nil {
		text = extMsg.GetText()
	}

	if text == "" {
		log.Println("Could not extract any valid text from the message, ignoring")
		return
	}

	log.Printf("Received valid text message from %s: %s", senderJID, text)
	cleanedText := strings.TrimSpace(text)

	if strings.HasPrefix(cleanedText, "/lang") {
		h.handleLangCommand(cleanedText, senderJID, localizer)
	} else if cleanedText == "/reset" || cleanedText == "/newchat" {
		h.handleResetCommand(senderJID, chatJID)
	} else {
		h.handleGeminiQuery(cleanedText, chatJID, senderJID, localizer)
	}
}

func (h *BotHandler) handleImageMessage(img *proto.ImageMessage, chatJID types.JID, senderJID string, localizer *goi18n.Localizer) {
    log.Printf("Processing image message from %s", senderJID)
    h.Client.SendChatPresence(chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
    defer h.Client.SendChatPresence(chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

    imageData, err := h.Client.Download(context.Background(), img)
    if err != nil {
        log.Printf("Failed to download image from %s: %v", senderJID, err)
        return
    }

    userCaption := img.GetCaption()
    if userCaption == "" {
        userCaption = "Tolong jelaskan apa yang ada di dalam gambar ini."
    }

    imageAnalysisInstruction := "Anda adalah asisten AI yang bisa menganalisis gambar. Jelaskan isi gambar yang dikirim oleh pengguna secara detail."

    finalPrompt := userCaption
    if h.KnowledgeEnabled && h.Knowledge.Content != "" {
        finalPrompt = fmt.Sprintf("Main Instruction:\n%s\n\nGeneral Personality:\n\"\"\"\n%s\n\"\"\"\n\nUser's Question about the image:\n%s", imageAnalysisInstruction, h.Knowledge.Content, userCaption)
    }

    mimeType := img.GetMimetype()
    response, err := h.Gemini.GenerateContentWithImage(finalPrompt, mimeType, imageData)
    if err != nil {
        log.Printf("Error from Gemini Vision API for user %s: %v", senderJID, err)
        errorMsg, _ := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "error_gemini"})
        h.sendMessage(chatJID, errorMsg)
        return
    }

    log.Printf("Received vision response from Gemini for %s, sending reply", senderJID)
    h.sendMessage(chatJID, response)

    h.DB.AddMessageToHistory(senderJID, "user", "[User sent an image] "+img.GetCaption())
    h.DB.AddMessageToHistory(senderJID, "model", response)
}


func (h *BotHandler) sendLocation(recipient types.JID) {
	if h.StoreLatitude == 0 || h.StoreLongitude == 0 {
		log.Println("Store location is not configured")
		h.sendMessage(recipient, "Maaf, lokasi toko belum diatur.")
		return
	}
	
	lat := h.StoreLatitude
	lon := h.StoreLongitude

	msg := &proto.Message{
		LocationMessage: &proto.LocationMessage{
			DegreesLatitude:  &lat,
			DegreesLongitude: &lon,
		},
	}
	
	_, err := h.Client.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		log.Printf("Failed to send location to %s: %v", recipient, err)
	} else {
		log.Printf("Sent location to %s", recipient)
	}
}

func (h *BotHandler) sendImage(recipient types.JID, imagePath, caption string) {
	if imagePath == "" {
		log.Println("Image path is not configured")
		h.sendMessage(recipient, "Maaf, file gambar belum diatur.")
		return
	}

	data, err := os.ReadFile(imagePath)
	if err != nil {
		log.Printf("Failed to read image file %s: %v", imagePath, err)
		h.sendMessage(recipient, "Maaf, terjadi kesalahan saat membaca file gambar.")
		return
	}

	uploaded, err := h.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		log.Printf("Failed to upload image: %v", err)
		h.sendMessage(recipient, "Maaf, terjadi kesalahan saat mengunggah gambar.")
		return
	}

	mimetype := http.DetectContentType(data)
	msg := &proto.Message{
		ImageMessage: &proto.ImageMessage{
			Caption:       &caption,
			Mimetype:      &mimetype,
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		},
	}

	_, err = h.Client.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		log.Printf("Failed to send image to %s: %v", recipient, err)
	} else {
		log.Printf("Sent image to %s", recipient)
	}
}

func (h *BotHandler) handleResetCommand(senderJID string, chatJID types.JID) {
	err := h.DB.DeleteConversationHistory(senderJID)
	if err == nil {
		h.sendMessage(chatJID, "Conversation history has been reset.")
	} else {
		h.sendMessage(chatJID, "Failed to reset conversation history.")
	}
}


func (h *BotHandler) handleLangCommand(text, senderJID string, localizer *goi18n.Localizer) {
	parts := strings.Split(text, " ")
	if len(parts) < 2 {
		return
	}
	lang := strings.ToLower(parts[1])

	recipientJID, err := types.ParseJID(senderJID)
	if err != nil {
		log.Printf("Failed to parse sender JID %s: %v", senderJID, err)
		return
	}

	if lang != "en" && lang != "id" {
		msg, _ := localizer.Localize(&goi18n.LocalizeConfig{
			MessageID: "lang_not_found",
			TemplateData: map[string]string{
				"Lang": lang,
			},
		})
		h.sendMessage(recipientJID, msg)
		return
	}

	err = h.DB.SetUserLang(senderJID, lang)
	if err != nil {
		log.Printf("Error setting language for %s: %v", senderJID, err)
		return
	}

	newLocalizer := goi18n.NewLocalizer(h.Bundle, lang)
	msg, _ := newLocalizer.Localize(&goi18n.LocalizeConfig{MessageID: "lang_updated"})
	h.sendMessage(recipientJID, msg)
	log.Printf("User %s language updated to %s", senderJID, lang)
}

func (h *BotHandler) handleGeminiQuery(prompt string, chatJID types.JID, senderJID string, localizer *goi18n.Localizer) {
    log.Printf("Forwarding message from %s to Gemini", senderJID)

    h.Client.SendChatPresence(chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
    defer h.Client.SendChatPresence(chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

    historyFromDB := h.DB.GetConversationHistory(senderJID)
    var geminiHistory []*genai.Content

    for _, msg := range historyFromDB {
        geminiHistory = append(geminiHistory, &genai.Content{
            Parts: []genai.Part{genai.Text(msg.Message)},
            Role:  msg.Role,
        })
    }

    finalPrompt := prompt
    if h.KnowledgeEnabled && h.Knowledge.Content != "" {
        knowledgePrompt := fmt.Sprintf("Use this personality to answer:\n\"\"\"\n%s\n\"\"\"\n\nUser's Question: %s", h.Knowledge.Content, prompt)
        finalPrompt = knowledgePrompt
    }

    geminiHistory = append(geminiHistory, &genai.Content{
        Parts: []genai.Part{genai.Text(finalPrompt)},
        Role:  "user",
    })

    response, err := h.Gemini.GenerateContent(geminiHistory)
    if err != nil {
        log.Printf("Error from Gemini API for user %s: %v", senderJID, err)
        errorMsg, _ := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "error_gemini"})
        h.sendMessage(chatJID, errorMsg)
        return
    }

    log.Printf("Received response from Gemini for %s", senderJID)
    h.sendMessage(chatJID, response)
    h.DB.AddMessageToHistory(senderJID, "user", prompt)
    h.DB.AddMessageToHistory(senderJID, "model", response)
}


func (h *BotHandler) sendMessage(recipient types.JID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.Client.SendMessage(ctx, recipient, &proto.Message{
		Conversation: &message,
	})
	if err != nil {
		log.Printf("Error sending message to %s: %v", recipient.String(), err)
	} else {
		log.Printf("Sent message to %s", recipient.String())
	}
}