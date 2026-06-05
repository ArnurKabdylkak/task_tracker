// Package scheduler выполняет фоновые ежедневные напоминания.
package scheduler

import (
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/config"
	"tasktracker/internal/domain"
	"tasktracker/internal/timeutil"
)

const morningText = "🔔 Доброе утро! Не забудьте отписаться по задачам на сегодня.\n\n" +
	"Отправьте список задач сообщением или используйте /add. Посмотреть план — /today."

const followupText = "⏰ Напоминаем: вы ещё не отписались по задачам на сегодня.\n\n" +
	"Отправьте список задач сообщением или используйте /add."

// Sender — минимальный контракт отправки, нужный планировщику.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Scheduler рассылает напоминания по расписанию.
type Scheduler struct {
	bot   Sender
	store domain.Repository
	cfg   *config.Config
}

// New создаёт планировщик.
func New(bot Sender, store domain.Repository, cfg *config.Config) *Scheduler {
	return &Scheduler{bot: bot, store: store, cfg: cfg}
}

// Start запускает фоновые задания: утреннее напоминание всем
// и дневное — тем, кто не отписался.
func (s *Scheduler) Start() {
	go s.scheduleDaily(s.cfg.ReminderHour, s.cfg.ReminderMin, "утреннее напоминание", s.sendMorning)
	go s.scheduleDaily(s.cfg.FollowupHour, s.cfg.FollowupMin, "напоминание не отписавшимся", s.sendFollowup)
}

// scheduleDaily каждый день в hour:min вызывает fn.
func (s *Scheduler) scheduleDaily(hour, min int, name string, fn func()) {
	for {
		d := untilNext(s.cfg.Location, hour, min)
		log.Printf("[%s] следующий запуск через %s", name, d.Truncate(time.Second))
		<-time.After(d)
		fn()
	}
}

func untilNext(loc *time.Location, hour, min int) time.Duration {
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return time.Until(next)
}

// sendMorning рассылает утреннее напоминание всем сотрудникам.
func (s *Scheduler) sendMorning() {
	emps, err := s.store.GetUsersByRole(domain.RoleEmployee)
	if err != nil {
		log.Printf("sendMorning: %v", err)
		return
	}
	for _, e := range emps {
		if _, err := s.bot.Send(tgbotapi.NewMessage(e.ChatID, morningText)); err != nil {
			log.Printf("morning -> %d: %v", e.TgID, err)
		}
	}
	log.Printf("Утренние напоминания отправлены: %d", len(emps))
}

// sendFollowup пингует сотрудников без задач на сегодня и шлёт сводку начальникам.
func (s *Scheduler) sendFollowup() {
	date := timeutil.DayStr(time.Now().In(s.cfg.Location))
	missing, err := s.store.EmployeesWithoutTasks(date)
	if err != nil {
		log.Printf("sendFollowup: %v", err)
		return
	}
	if len(missing) == 0 {
		log.Printf("Все сотрудники отписались, доп. напоминания не нужны")
		return
	}

	// Напоминаем самим не отписавшимся.
	for _, e := range missing {
		if _, err := s.bot.Send(tgbotapi.NewMessage(e.ChatID, followupText)); err != nil {
			log.Printf("followup -> %d: %v", e.TgID, err)
		}
	}

	// Сводка начальникам.
	var b strings.Builder
	fmt.Fprintf(&b, "⚠️ Не отписались на сегодня (%d):\n", len(missing))
	for i, e := range missing {
		name := html.EscapeString(e.FullName)
		if e.Username != "" {
			name += " (@" + html.EscapeString(e.Username) + ")"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
	}
	bosses, _ := s.store.GetUsersByRole(domain.RoleBoss)
	for _, boss := range bosses {
		m := tgbotapi.NewMessage(boss.ChatID, b.String())
		m.ParseMode = tgbotapi.ModeHTML
		if _, err := s.bot.Send(m); err != nil {
			log.Printf("followup boss -> %d: %v", boss.TgID, err)
		}
	}
	log.Printf("Доп. напоминания: %d сотрудникам, сводка %d начальникам", len(missing), len(bosses))
}
