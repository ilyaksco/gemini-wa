package bot

import (
	"context"
	"gemini-whatsapp-bot/internal/db"
	geminiClient "gemini-whatsapp-bot/pkg/gemini"
	"log"
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

	userLang := h.DB.GetUserLang(senderJID)
	localizer := goi18n.NewLocalizer(h.Bundle, userLang)
	
	cleanedText := strings.TrimSpace(text)

	if strings.HasPrefix(cleanedText, "/lang") {
		h.handleLangCommand(cleanedText, senderJID, localizer)
	} else if cleanedText == "/reset" || cleanedText == "/newchat" {
		h.handleResetCommand(senderJID, msg.Info.Chat)
	} else {
		h.handleGeminiQuery(cleanedText, msg.Info.Chat, senderJID, localizer)
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
	log.Printf("Forwarding message from %s to Gemini with history", senderJID)
	
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

	geminiHistory = append(geminiHistory, &genai.Content{
		Parts: []genai.Part{genai.Text(prompt)},
		Role:  "user",
	})


	response, err := h.Gemini.GenerateContent(geminiHistory)
	if err != nil {
		log.Printf("Error from Gemini API for user %s: %v", senderJID, err)
		errorMsg, _ := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "error_gemini"})
		h.sendMessage(chatJID, errorMsg)
		return
	}

	log.Printf("Received response from Gemini for %s, sending reply", senderJID)
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