package bot

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/config"
	"tasktracker/internal/domain"
	"tasktracker/internal/storage"
)

// captureSender — мок Sender: запоминает отправленные сообщения вместо вызова Telegram.
type captureSender struct {
	texts []string
	docs  int
}

func (c *captureSender) Send(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		c.texts = append(c.texts, m.Text)
	case tgbotapi.EditMessageTextConfig:
		c.texts = append(c.texts, m.Text)
	case tgbotapi.DocumentConfig:
		c.docs++
	}
	return tgbotapi.Message{}, nil
}

func (c *captureSender) Request(tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (c *captureSender) last() string {
	if len(c.texts) == 0 {
		return ""
	}
	return c.texts[len(c.texts)-1]
}

// newTestHandler собирает хендлер на in-memory БД и мок-отправителе.
func newTestHandler(t *testing.T, bossID int64) (*Handler, *captureSender, domain.Repository) {
	t.Helper()
	store, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.Config{
		BossIDs:  map[int64]bool{bossID: true},
		Location: time.UTC,
	}
	sender := &captureSender{}
	return NewHandler(sender, store, cfg), sender, store
}

func msgFrom(id int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		From: &tgbotapi.User{ID: id, FirstName: "Тест"},
		Chat: &tgbotapi.Chat{ID: id},
		Text: text,
	}
}

func cmdFrom(id int64, command, args string) *tgbotapi.Message {
	text := "/" + command
	if args != "" {
		text += " " + args
	}
	m := msgFrom(id, text)
	m.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len("/" + command)}}
	return m
}

// Свободный текст превращается в задачи на сегодня, в ответ приходит план.
func TestPlainTextAddsTasks(t *testing.T) {
	h, sender, store := newTestHandler(t, 999)

	h.handleMessage(msgFrom(1, "Сделать отчёт\n- Созвон\nРевью PR"))

	tasks, _ := store.TasksByDate(1, h.now().Format("2006-01-02"))
	if len(tasks) != 3 {
		t.Fatalf("ожидалось 3 задачи, получили %d", len(tasks))
	}
	if !strings.Contains(sender.last(), "Созвон") {
		t.Fatalf("в ответе нет добавленной задачи: %q", sender.last())
	}
	// Префикс списка "- " должен быть срезан.
	for _, tk := range tasks {
		if strings.HasPrefix(tk.Text, "- ") {
			t.Fatalf("префикс списка не срезан: %q", tk.Text)
		}
	}
}

// Команда начальника недоступна обычному сотруднику.
func TestBossCommandDeniedForEmployee(t *testing.T) {
	h, sender, _ := newTestHandler(t, 999)

	h.handleMessage(cmdFrom(1, "pending", "")) // 1 — не начальник

	if !strings.Contains(sender.last(), "только начальнику") {
		t.Fatalf("ожидался отказ в доступе, получили: %q", sender.last())
	}
}

// Та же команда у начальника отрабатывает штатно.
func TestBossCommandAllowedForBoss(t *testing.T) {
	h, sender, _ := newTestHandler(t, 7) // 7 — начальник

	h.handleMessage(cmdFrom(7, "pending", ""))

	if strings.Contains(sender.last(), "только начальнику") {
		t.Fatalf("начальнику отказано в доступе: %q", sender.last())
	}
	if !strings.Contains(sender.last(), "отписали") {
		t.Fatalf("неожиданный ответ на /pending: %q", sender.last())
	}
}

// Нажатие toggle переключает статус задачи владельца.
func TestToggleCallback(t *testing.T) {
	h, _, store := newTestHandler(t, 999)
	id, _ := store.AddTask(1, "Задача", h.now().Format("2006-01-02"))

	h.handleCallback(&tgbotapi.CallbackQuery{
		ID:      "x",
		From:    &tgbotapi.User{ID: 1},
		Message: &tgbotapi.Message{MessageID: 10, Chat: &tgbotapi.Chat{ID: 1}},
		Data:    "toggle:" + strconv.FormatInt(id, 10),
	})

	tasks, _ := store.TasksByDate(1, h.now().Format("2006-01-02"))
	if len(tasks) != 1 || !tasks[0].Done() {
		t.Fatalf("задача не переключилась в done: %+v", tasks)
	}
}
