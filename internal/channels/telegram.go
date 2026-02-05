package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/hkuds/ubot/internal/bus"
	"github.com/hkuds/ubot/internal/config"
)

// TelegramChannel implements the Channel interface for Telegram messaging.
type TelegramChannel struct {
	BaseChannel
	token   string
	bot     *tgbotapi.BotAPI
	groqKey string // for voice transcription

	// chatIDs maps string chat IDs to int64 for message sending
	chatIDs map[string]int64
	chatMu  sync.RWMutex

	// cancel function for stopping the update loop
	cancel context.CancelFunc
}

// NewTelegramChannel creates a new Telegram channel instance.
func NewTelegramChannel(cfg config.TelegramConfig, msgBus *bus.MessageBus, groqKey string) *TelegramChannel {
	return &TelegramChannel{
		BaseChannel: NewBaseChannel("telegram", msgBus, cfg.AllowFrom),
		token:       cfg.Token,
		groqKey:     groqKey,
		chatIDs:     make(map[string]int64),
	}
}

// Start begins listening for Telegram updates.
func (c *TelegramChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return fmt.Errorf("telegram channel is already running")
	}

	// Create bot API with token
	bot, err := tgbotapi.NewBotAPI(c.token)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}
	c.bot = bot

	log.Printf("Telegram bot authorized as @%s", bot.Self.UserName)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// Configure update settings
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60 // Long polling timeout

	updates := bot.GetUpdatesChan(u)

	c.setRunning(true)

	// Subscribe to outbound messages for this channel
	c.getBus().SubscribeOutbound("telegram", func(msg bus.OutboundMessage) {
		if err := c.Send(msg); err != nil {
			log.Printf("Error sending Telegram message: %v", err)
		}
	})

	// Start processing updates in a goroutine
	go c.processUpdates(ctx, updates)

	return nil
}

// processUpdates handles incoming Telegram updates.
func (c *TelegramChannel) processUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram update processing stopped")
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			c.handleMessage(update.Message)
		}
	}
}

// handleMessage processes an individual Telegram message.
func (c *TelegramChannel) handleMessage(msg *tgbotapi.Message) {
	// Build sender ID (user_id|username if available)
	senderID := strconv.FormatInt(msg.From.ID, 10)
	if msg.From.UserName != "" {
		senderID = senderID + "|" + msg.From.UserName
	}

	// Check if sender is allowed
	if !c.IsAllowed(senderID) {
		log.Printf("Telegram message from unauthorized sender: %s", senderID)
		return
	}

	// Store chat ID mapping
	chatIDStr := strconv.FormatInt(msg.Chat.ID, 10)
	c.chatMu.Lock()
	c.chatIDs[chatIDStr] = msg.Chat.ID
	c.chatMu.Unlock()

	// Build metadata
	metadata := make(map[string]interface{})
	metadata["messageId"] = msg.MessageID
	metadata["chatType"] = msg.Chat.Type
	if msg.From.FirstName != "" {
		metadata["firstName"] = msg.From.FirstName
	}
	if msg.From.LastName != "" {
		metadata["lastName"] = msg.From.LastName
	}
	if msg.From.UserName != "" {
		metadata["username"] = msg.From.UserName
	}

	var content string
	var media []string

	// Handle different message types
	switch {
	case msg.Voice != nil:
		// Handle voice message with transcription
		transcription, err := c.transcribeVoice(msg.Voice)
		if err != nil {
			log.Printf("Failed to transcribe voice message: %v", err)
			content = "[Voice message - transcription failed]"
		} else {
			content = transcription
			metadata["originalType"] = "voice"
		}

	case msg.Photo != nil && len(msg.Photo) > 0:
		// Get the highest resolution photo
		photo := msg.Photo[len(msg.Photo)-1]
		media = append(media, photo.FileID)
		content = msg.Caption
		metadata["originalType"] = "photo"

	case msg.Document != nil:
		media = append(media, msg.Document.FileID)
		content = msg.Caption
		metadata["originalType"] = "document"
		metadata["fileName"] = msg.Document.FileName
		metadata["mimeType"] = msg.Document.MimeType

	case msg.Text != "":
		content = msg.Text

	default:
		// Handle other message types as generic content
		if msg.Caption != "" {
			content = msg.Caption
		}
	}

	// Publish to message bus
	c.publishInbound(senderID, chatIDStr, content, media, metadata)
}

// transcribeVoice transcribes a voice message using Groq API.
func (c *TelegramChannel) transcribeVoice(voice *tgbotapi.Voice) (string, error) {
	if c.groqKey == "" {
		return "", fmt.Errorf("Groq API key not configured for voice transcription")
	}

	// Get the voice file from Telegram
	fileConfig := tgbotapi.FileConfig{FileID: voice.FileID}
	file, err := c.bot.GetFile(fileConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get voice file: %w", err)
	}

	// Download the file
	fileURL := file.Link(c.token)
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download voice file: %w", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read voice data: %w", err)
	}

	// Send to Groq for transcription
	return c.transcribeWithGroq(audioData, "audio.ogg")
}

// transcribeWithGroq sends audio data to Groq's Whisper API for transcription.
func (c *TelegramChannel) transcribeWithGroq(audioData []byte, filename string) (string, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the audio file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add the model parameter
	if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request to Groq
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.groqKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send transcription request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode transcription response: %w", err)
	}

	return result.Text, nil
}

// Stop gracefully shuts down the Telegram channel.
func (c *TelegramChannel) Stop() error {
	if !c.IsRunning() {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	if c.bot != nil {
		c.bot.StopReceivingUpdates()
	}

	c.setRunning(false)
	log.Println("Telegram channel stopped")
	return nil
}

// Send delivers an outbound message through Telegram.
func (c *TelegramChannel) Send(msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("telegram channel is not running")
	}

	// Parse chat ID to int64
	chatID, err := c.getChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Convert markdown to Telegram HTML
	htmlContent := MarkdownToTelegramHTML(msg.Content)

	// Create and send message with HTML parsing
	telegramMsg := tgbotapi.NewMessage(chatID, htmlContent)
	telegramMsg.ParseMode = tgbotapi.ModeHTML

	// Set reply-to if specified
	if msg.ReplyTo != "" {
		if replyID, err := strconv.Atoi(msg.ReplyTo); err == nil {
			telegramMsg.ReplyToMessageID = replyID
		}
	}

	_, err = c.bot.Send(telegramMsg)
	if err != nil {
		// Fallback to plain text if HTML fails
		log.Printf("HTML message failed, falling back to plain text: %v", err)
		telegramMsg.ParseMode = ""
		telegramMsg.Text = StripMarkdown(msg.Content)
		_, err = c.bot.Send(telegramMsg)
	}

	return err
}

// getChatID retrieves the int64 chat ID from a string ID.
func (c *TelegramChannel) getChatID(chatIDStr string) (int64, error) {
	// First check our cache
	c.chatMu.RLock()
	if chatID, ok := c.chatIDs[chatIDStr]; ok {
		c.chatMu.RUnlock()
		return chatID, nil
	}
	c.chatMu.RUnlock()

	// Parse directly if not in cache
	chatIDStr = strings.TrimSpace(chatIDStr)
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse chat ID '%s': %w", chatIDStr, err)
	}

	// Store in cache for future use
	c.chatMu.Lock()
	c.chatIDs[chatIDStr] = chatID
	c.chatMu.Unlock()

	return chatID, nil
}
