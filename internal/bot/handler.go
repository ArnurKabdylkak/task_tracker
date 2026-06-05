// Package bot реализует Telegram-слой: роутинг апдейтов, команды и представление.
package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/config"
	"tasktracker/internal/domain"
	"tasktracker/internal/timeutil"
)

const (
	maxTasksPerMessage = 50
	maxTaskLen         = 500
)

// Loc — псевдоним для таймзоны, используется слоем представления.
type Loc = *time.Location

// Handler связывает бота, хранилище и конфиг.
type Handler struct {
	bot   Sender
	store domain.Repository
	cfg   *config.Config
}

// NewHandler собирает обработчик апдейтов.
func NewHandler(bot Sender, store domain.Repository, cfg *config.Config) *Handler {
	return &Handler{bot: bot, store: store, cfg: cfg}
}

// Handle — точка входа для каждого обновления.
func (h *Handler) Handle(u tgbotapi.Update) {
	if u.CallbackQuery != nil {
		h.handleCallback(u.CallbackQuery)
		return
	}
	if u.Message != nil {
		h.handleMessage(u.Message)
	}
}

func (h *Handler) handleMessage(msg *tgbotapi.Message) {
	user, err := h.ensureUser(msg)
	if err != nil {
		log.Printf("ensureUser: %v", err)
		return
	}

	if msg.IsCommand() {
		h.handleCommand(user, msg)
		return
	}

	// Любой обычный текст трактуем как список задач на сегодня.
	if strings.TrimSpace(msg.Text) == "" {
		return
	}
	added := h.addTasks(user, msg.Text)
	if added == 0 {
		h.send(user.ChatID, "Не понял сообщение. Отправьте задачи списком (каждая с новой строки) или наберите /help")
		return
	}
	h.sendToday(user.ChatID, user.TgID)
}

func (h *Handler) handleCommand(user *domain.User, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start", "help":
		h.send(user.ChatID, h.helpText(user))
	case "id":
		h.send(user.ChatID, fmt.Sprintf("Ваш Telegram ID: <code>%d</code>", user.TgID))
	case "add":
		args := strings.TrimSpace(msg.CommandArguments())
		if args == "" {
			h.send(user.ChatID, "Напишите задачи после команды, например:\n/add Созвон с клиентом\nили просто отправьте список задач сообщением.")
			return
		}
		h.addTasks(user, args)
		h.sendToday(user.ChatID, user.TgID)
	case "today":
		h.sendToday(user.ChatID, user.TgID)
	case "yesterday":
		h.cmdDay(user, timeutil.DayStr(h.now().AddDate(0, 0, -1)), "📅 Вчерашний план")
	case "week":
		h.cmdWeek(user)
	case "all":
		h.cmdAll(user, msg.CommandArguments())
	case "team":
		h.cmdTeam(user)
	case "export":
		h.cmdExport(user, msg.CommandArguments())
	case "pending":
		h.cmdPending(user)
	default:
		h.send(user.ChatID, "Неизвестная команда. Наберите /help")
	}
}

// ensureUser регистрирует/обновляет пользователя и определяет его роль.
func (h *Handler) ensureUser(msg *tgbotapi.Message) (*domain.User, error) {
	role := domain.RoleEmployee
	if h.cfg.BossIDs[msg.From.ID] {
		role = domain.RoleBoss
	}
	name := strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	if name == "" {
		name = msg.From.UserName
	}
	if name == "" {
		name = fmt.Sprintf("id%d", msg.From.ID)
	}
	u := domain.User{
		TgID:     msg.From.ID,
		ChatID:   msg.Chat.ID,
		Username: msg.From.UserName,
		FullName: name,
		Role:     role,
	}
	if err := h.store.UpsertUser(u); err != nil {
		return nil, err
	}
	return &u, nil
}

// addTasks разбивает текст на строки и добавляет каждую как отдельную задачу на сегодня.
func (h *Handler) addTasks(user *domain.User, text string) int {
	today := timeutil.DayStr(h.now())
	count := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "• ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = timeutil.Short(line, maxTaskLen)
		if _, err := h.store.AddTask(user.TgID, line, today); err != nil {
			log.Printf("AddTask: %v", err)
			continue
		}
		count++
		if count >= maxTasksPerMessage {
			break
		}
	}
	return count
}

// sendToday отправляет план на сегодня с кнопками управления.
func (h *Handler) sendToday(chatID, tgID int64) {
	text, kb := h.renderToday(tgID)
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	if kb != nil {
		m.ReplyMarkup = *kb
	}
	if _, err := h.bot.Send(m); err != nil {
		log.Printf("sendToday: %v", err)
	}
}

// renderToday строит текст плана на сегодня и клавиатуру.
func (h *Handler) renderToday(tgID int64) (string, *tgbotapi.InlineKeyboardMarkup) {
	today := timeutil.DayStr(h.now())
	tasks, err := h.store.TasksByDate(tgID, today)
	if err != nil {
		log.Printf("TasksByDate: %v", err)
	}
	header := "🗓 План на сегодня (" + timeutil.RuDate(today, h.cfg.Location) + ")"
	text := buildTaskList(header, tasks)
	if len(tasks) == 0 {
		text += "\n\nОтправьте задачи списком или используйте /add."
		return text, nil
	}
	return text, todayKeyboard(tasks)
}

func (h *Handler) handleCallback(cb *tgbotapi.CallbackQuery) {
	// Подтверждаем нажатие, чтобы убрать "часики".
	defer h.bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	parts := strings.SplitN(cb.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}
	uid := cb.From.ID

	switch parts[0] {
	case "toggle":
		if err := h.store.ToggleTask(id, uid); err != nil {
			log.Printf("ToggleTask: %v", err)
		}
	case "del":
		if err := h.store.DeleteTask(id, uid); err != nil {
			log.Printf("DeleteTask: %v", err)
		}
	default:
		return
	}

	text, kb := h.renderToday(uid)
	edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	if kb != nil {
		edit.ReplyMarkup = kb
	} else {
		empty := tgbotapi.NewInlineKeyboardMarkup()
		edit.ReplyMarkup = &empty
	}
	if _, err := h.bot.Send(edit); err != nil {
		log.Printf("edit: %v", err)
	}
}

func (h *Handler) now() time.Time { return time.Now().In(h.cfg.Location) }
