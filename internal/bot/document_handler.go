package bot

import (
	"context"
	"log"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

func (h *BotHandler) handleDocumentMessage(doc *proto.DocumentMessage, chatJID types.JID, senderJID string, historyJID string, isGroup bool, localizer *goi18n.Localizer) {
	log.Printf("Processing document message from %s", senderJID)

	mimeType := doc.GetMimetype()
	if mimeType != "application/pdf" {
		log.Printf("Received non-PDF document from %s with MIME type %s, ignoring", senderJID, mimeType)
		h.sendMessage(chatJID, "Sorry, I can only process PDF documents at the moment.")
		return
	}

	userCaption := doc.GetCaption()

	if isGroup {
		if !strings.HasPrefix(userCaption, "/ask") && !strings.HasPrefix(userCaption, "/ai") {
			log.Printf("Document in group from %s without trigger, ignoring", senderJID)
			return
		}
		if strings.HasPrefix(userCaption, "/ask ") {
			userCaption = strings.TrimSpace(strings.TrimPrefix(userCaption, "/ask "))
		} else if strings.HasPrefix(userCaption, "/ai ") {
			userCaption = strings.TrimSpace(strings.TrimPrefix(userCaption, "/ai "))
		}
	}
	
	h.Client.SendChatPresence(chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	defer h.Client.SendChatPresence(chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

	pdfData, err := h.Client.Download(context.Background(), doc)
	if err != nil {
		log.Printf("Failed to download document from %s: %v", senderJID, err)
		return
	}

	if userCaption == "" {
		userCaption = "Please summarize this document."
	}
	userName := ""

	response, err := h.Gemini.GenerateContentWithDocument(userCaption, mimeType, pdfData)
	if err != nil {
		log.Printf("Error from Gemini Document API for user %s: %v", senderJID, err)
		errorMsg, _ := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "error_gemini"})
		h.sendMessage(chatJID, errorMsg)
		return
	}

	log.Printf("Received document response from Gemini for %s, sending reply", senderJID)
	h.sendMessage(chatJID, response)

	h.DB.AddMessageToHistory(historyJID, "user", "[User sent a PDF] "+userCaption, userName)
	h.DB.AddMessageToHistory(historyJID, "model", response, "")
}