package gemini

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Client struct {
	keys            []string
	currentKeyIndex int
	mu              sync.Mutex
}

func New(apiKeys []string) *Client {
	if len(apiKeys) == 0 {
		log.Fatal("Cannot create Gemini client with no API keys")
	}
	log.Println("Gemini client initialized with key rotation enabled")
	return &Client{
		keys: apiKeys,
	}
}

func (c *Client) GenerateContent(history []*genai.Content) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(history) == 0 {
		return "", errors.New("empty history")
	}

	totalKeys := len(c.keys)
	for i := 0; i < totalKeys; i++ {
		key := c.keys[c.currentKeyIndex]

		ctx := context.Background()
		client, err := genai.NewClient(ctx, option.WithAPIKey(key))
		if err != nil {
			log.Printf("Failed to create Gemini client with key index %d: %v", c.currentKeyIndex, err)
			c.rotateToNextKey()
			continue
		}

		model := client.GenerativeModel("gemini-2.5-flash-lite")
		cs := model.StartChat()
		if len(history) > 1 {
			cs.History = history[0 : len(history)-1]
		}

		lastPrompt := history[len(history)-1].Parts[0]
		resp, err := cs.SendMessage(ctx, lastPrompt)
		client.Close()

		if err != nil {
			if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "429") {
				log.Printf("API key at index %d is rate-limited. Rotating to next key.", c.currentKeyIndex)
				c.rotateToNextKey()
				continue
			}
			return "", err
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return "No response from model.", nil
		}

		return string(resp.Candidates[0].Content.Parts[0].(genai.Text)), nil
	}

	return "", errors.New("all Gemini API keys are rate-limited or invalid")
}

func (c *Client) GenerateContentWithImage(prompt string, mimeType string, imageData []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	totalKeys := len(c.keys)
	for i := 0; i < totalKeys; i++ {
		key := c.keys[c.currentKeyIndex]

		ctx := context.Background()
		client, err := genai.NewClient(ctx, option.WithAPIKey(key))
		if err != nil {
			log.Printf("Failed to create Gemini client with key index %d: %v", c.currentKeyIndex, err)
			c.rotateToNextKey()
			continue
		}

		model := client.GenerativeModel("gemini-2.5-flash")
		content := []genai.Part{
			genai.ImageData(mimeType, imageData),
			genai.Text(prompt),
		}

		resp, err := model.GenerateContent(ctx, content...)
		client.Close()

		if err != nil {
			if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "429") {
				log.Printf("API key at index %d is rate-limited. Rotating to next key.", c.currentKeyIndex)
				c.rotateToNextKey()
				continue
			}
			return "", err
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return "No response from model.", nil
		}

		return string(resp.Candidates[0].Content.Parts[0].(genai.Text)), nil
	}

	return "", errors.New("all Gemini API keys are rate-limited or invalid")
}


func (c *Client) rotateToNextKey() {
	totalKeys := len(c.keys)
	c.currentKeyIndex = (c.currentKeyIndex + 1) % totalKeys
	log.Printf("Rotated to next key index: %d", c.currentKeyIndex)
}