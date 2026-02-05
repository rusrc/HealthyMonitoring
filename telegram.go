package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// SendTelegramMessage отправляет текст в чат Telegram через Bot API.
// Требуются переменные окружения (загружаются из .env при старте):
//   - TELEGRAM_BOT_TOKEN — токен бота (получить у @BotFather)
//   - TELEGRAM_CHAT_ID — ID чата (как получить: напиши боту любое сообщение,
//     затем открой в браузере https://api.telegram.org/bot<ТОКЕН>/getUpdates —
//     в ответе в message.chat.id будет твой chat_id)
//
// Пример: в коде вызвать SendTelegramMessage("сайт https://example.com вернул 200")
func SendTelegramMessage(text string) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		return fmt.Errorf("telegram: задайте TELEGRAM_BOT_TOKEN и TELEGRAM_CHAT_ID в .env файле")
	}

	url := "https://api.telegram.org/bot" + token + "/sendMessage"
	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "", // можно "HTML" или "Markdown" при необходимости
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("telegram encode: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api: status %d", resp.StatusCode)
	}
	return nil
}
