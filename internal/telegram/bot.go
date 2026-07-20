package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/reinh2/docmind/internal/chat"
)

// Bot uses the same chat use case as the widget, preserving one audit trail.
type Bot struct {
	token         string
	client        *http.Client
	chat          *chat.Service
	log           *slog.Logger
	offset        int64
	conversations map[int64]string
}

func New(token string, service *chat.Service, log *slog.Logger) *Bot {
	return &Bot{token: token, client: &http.Client{Timeout: 35 * time.Second}, chat: service, log: log, conversations: map[int64]string{}}
}
func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	for {
		if err := b.poll(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.log.Error("Telegram poll failed", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
}
func (b *Bot) poll(ctx context.Context) error {
	var response struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
			Message  struct {
				Chat struct {
					ID int64 `json:"id"`
				}
				Text string `json:"text"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := b.call(ctx, "getUpdates", map[string]any{"offset": b.offset, "timeout": 25, "allowed_updates": []string{"message"}}, &response); err != nil {
		return err
	}
	for _, u := range response.Result {
		b.offset = u.UpdateID + 1
		text := strings.TrimSpace(u.Message.Text)
		if text != "" {
			b.handle(ctx, u.Message.Chat.ID, text)
		}
	}
	return nil
}
func (b *Bot) handle(ctx context.Context, chatID int64, text string) {
	session := "telegram:" + strconv.FormatInt(chatID, 10)
	if text == "/start" {
		_ = b.send(ctx, chatID, "Hello! Ask me about our company documents. I answer only from the knowledge base.")
		return
	}
	if text == "/human" {
		conversationID := b.conversations[chatID]
		if conversationID == "" {
			_ = b.send(ctx, chatID, "Ask a question first, then I can pass the conversation to a person.")
			return
		}
		if err := b.chat.Escalate(ctx, conversationID, "customer requested a human via Telegram"); err != nil {
			_ = b.send(ctx, chatID, "I couldn’t notify the team. Please try again shortly.")
			return
		}
		_ = b.send(ctx, chatID, "Your request has been sent to our team.")
		return
	}
	result, err := b.chat.Ask(ctx, session, "telegram", text, nil)
	if err != nil {
		b.log.Error("Telegram chat failed", "error", err)
		_ = b.send(ctx, chatID, "I’m sorry, I can’t answer right now. Please try again shortly.")
		return
	}
	b.conversations[chatID] = result.ConversationID
	answer := result.Answer
	if len(result.Sources) > 0 {
		answer += "\n\nSources:"
		for _, source := range result.Sources {
			answer += "\n• " + source.Document
			if source.Section != "" {
				answer += " — " + source.Section
			}
		}
	}
	_ = b.send(ctx, chatID, answer)
}
func (b *Bot) send(ctx context.Context, chatID int64, text string) error {
	if len(text) > 4000 {
		text = text[:4000]
	}
	var out struct {
		OK bool `json:"ok"`
	}
	return b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, &out)
}
func (b *Bot) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+b.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
