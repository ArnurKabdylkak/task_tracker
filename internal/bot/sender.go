package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender — минимальный контракт отправки сообщений в Telegram.
// Реализуется *tgbotapi.BotAPI; за интерфейсом — чтобы мокать в тестах.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// send отправляет HTML-сообщение и логирует ошибку.
func (h *Handler) send(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	if _, err := h.bot.Send(m); err != nil {
		log.Printf("send: %v", err)
	}
}

// sendLong разбивает длинный текст на части по границам строк (лимит Telegram — 4096).
func (h *Handler) sendLong(chatID int64, text string) {
	for _, chunk := range splitLong(text) {
		h.send(chatID, chunk)
	}
}

// splitLong нарезает текст на куски по границам строк, не превышающие лимит.
func splitLong(text string) []string {
	const limit = 3900
	var out []string
	var b []byte
	flush := func() {
		if len(b) == 0 {
			return
		}
		out = append(out, string(b))
		b = b[:0]
	}
	for _, line := range splitLines(text) {
		if len(b)+len(line)+1 > limit {
			flush()
		}
		b = append(b, line...)
		b = append(b, '\n')
	}
	flush()
	return out
}
