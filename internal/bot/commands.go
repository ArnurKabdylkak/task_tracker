package bot

import (
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/domain"
	"tasktracker/internal/report"
	"tasktracker/internal/timeutil"
)

// requireBoss проверяет роль начальника; при отказе шлёт сообщение и возвращает false.
func (h *Handler) requireBoss(user *domain.User) bool {
	if user.Role != domain.RoleBoss {
		h.send(user.ChatID, "Эта команда доступна только начальнику.")
		return false
	}
	return true
}

// cmdDay показывает задачи за один конкретный день (только чтение).
func (h *Handler) cmdDay(user *domain.User, date, header string) {
	tasks, err := h.store.TasksByDate(user.TgID, date)
	if err != nil {
		log.Printf("cmdDay: %v", err)
	}
	full := header + " (" + timeutil.RuDate(date, h.cfg.Location) + ")"
	h.send(user.ChatID, buildTaskList(full, tasks))
}

// cmdWeek показывает план на неделю, сгруппированный по дням.
func (h *Handler) cmdWeek(user *domain.User) {
	from, to := timeutil.WeekRange(h.now())
	tasks, err := h.store.TasksBetween(user.TgID, from, to)
	if err != nil {
		log.Printf("cmdWeek: %v", err)
	}
	h.sendLong(user.ChatID, renderWeek(from, to, tasks, h.cfg.Location))
}

// cmdAll (только начальник) показывает задачи всех сотрудников за день.
// Аргумент: пусто/today/сегодня, yesterday/вчера или дата YYYY-MM-DD.
func (h *Handler) cmdAll(user *domain.User, arg string) {
	if !h.requireBoss(user) {
		return
	}

	date := timeutil.DayStr(h.now())
	label := "сегодня"
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "", "today", "сегодня":
	case "yesterday", "вчера":
		date = timeutil.DayStr(h.now().AddDate(0, 0, -1))
		label = "вчера"
	default:
		a := strings.TrimSpace(arg)
		if _, err := time.Parse("2006-01-02", a); err == nil {
			date = a
			label = a
		} else {
			h.send(user.ChatID, "Формат: /all, /all вчера или /all 2006-01-02")
			return
		}
	}

	emps, err := h.store.GetUsersByRole(domain.RoleEmployee)
	if err != nil {
		log.Printf("cmdAll: %v", err)
	}
	if len(emps) == 0 {
		h.send(user.ChatID, "Пока нет ни одного зарегистрированного сотрудника.")
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📋 Задачи команды (%s, %s)\n", label, timeutil.RuDate(date, h.cfg.Location))
	for _, e := range emps {
		tasks, _ := h.store.TasksByDate(e.TgID, date)
		fmt.Fprintf(&b, "\n👤 <b>%s</b>\n", userLabel(e))
		if len(tasks) == 0 {
			b.WriteString("   — не отписался\n")
			continue
		}
		done := 0
		for _, t := range tasks {
			box := "⬜"
			if t.Done() {
				box = "✅"
				done++
			}
			fmt.Fprintf(&b, "   %s %s\n", box, html.EscapeString(t.Text))
		}
		fmt.Fprintf(&b, "   <i>выполнено %d из %d</i>\n", done, len(tasks))
	}
	h.sendLong(user.ChatID, b.String())
}

// cmdExport (только начальник) формирует CSV-отчёт и отправляет файлом.
// Аргументы: пусто/сегодня, вчера, неделя, дата YYYY-MM-DD или диапазон "дата дата".
func (h *Handler) cmdExport(user *domain.User, arg string) {
	if !h.requireBoss(user) {
		return
	}
	from, to, label, ok := h.parseRange(arg)
	if !ok {
		h.send(user.ChatID, "Формат: /export, /export вчера, /export неделя, "+
			"/export 2006-01-02 или /export 2006-01-02 2006-01-07")
		return
	}
	rows, err := h.store.ReportRows(from, to)
	if err != nil {
		log.Printf("cmdExport: %v", err)
		h.send(user.ChatID, "Не удалось сформировать отчёт.")
		return
	}
	if len(rows) == 0 {
		h.send(user.ChatID, "За выбранный период задач нет.")
		return
	}
	data, err := report.BuildCSV(rows)
	if err != nil {
		log.Printf("BuildCSV: %v", err)
		h.send(user.ChatID, "Не удалось сформировать файл.")
		return
	}
	fileName := fmt.Sprintf("report_%s.csv", from)
	if from != to {
		fileName = fmt.Sprintf("report_%s_%s.csv", from, to)
	}
	doc := tgbotapi.NewDocument(user.ChatID, tgbotapi.FileBytes{Name: fileName, Bytes: data})
	doc.Caption = fmt.Sprintf("📊 Отчёт: %s (%d задач)", label, len(rows))
	if _, err := h.bot.Send(doc); err != nil {
		log.Printf("send doc: %v", err)
	}
}

// cmdPending (только начальник) показывает, кто ещё не отписался сегодня.
func (h *Handler) cmdPending(user *domain.User) {
	if !h.requireBoss(user) {
		return
	}
	date := timeutil.DayStr(h.now())
	missing, err := h.store.EmployeesWithoutTasks(date)
	if err != nil {
		log.Printf("cmdPending: %v", err)
	}
	if len(missing) == 0 {
		h.send(user.ChatID, "✅ Все сотрудники отписались на сегодня.")
		return
	}
	h.sendLong(user.ChatID, pendingList(missing))
}

// cmdTeam (только начальник) показывает список сотрудников.
func (h *Handler) cmdTeam(user *domain.User) {
	if !h.requireBoss(user) {
		return
	}
	emps, err := h.store.GetUsersByRole(domain.RoleEmployee)
	if err != nil {
		log.Printf("cmdTeam: %v", err)
	}
	h.sendLong(user.ChatID, teamList(emps))
}

// parseRange разбирает аргумент периода для отчёта.
func (h *Handler) parseRange(arg string) (from, to, label string, ok bool) {
	a := strings.ToLower(strings.TrimSpace(arg))
	now := h.now()
	switch a {
	case "", "today", "сегодня":
		d := timeutil.DayStr(now)
		return d, d, "сегодня", true
	case "yesterday", "вчера":
		d := timeutil.DayStr(now.AddDate(0, 0, -1))
		return d, d, "вчера", true
	case "week", "неделя":
		f, t := timeutil.WeekRange(now)
		return f, t, "неделя", true
	}
	parts := strings.Fields(a)
	const layout = "2006-01-02"
	if len(parts) == 1 {
		if _, err := time.Parse(layout, parts[0]); err == nil {
			return parts[0], parts[0], parts[0], true
		}
	}
	if len(parts) == 2 {
		_, e1 := time.Parse(layout, parts[0])
		_, e2 := time.Parse(layout, parts[1])
		if e1 == nil && e2 == nil {
			from, to = parts[0], parts[1]
			if from > to {
				from, to = to, from
			}
			return from, to, from + " — " + to, true
		}
	}
	return "", "", "", false
}

// helpText формирует текст помощи с учётом роли пользователя.
func (h *Handler) helpText(user *domain.User) string {
	common := strings.Join([]string{
		"<b>Таск-трекер</b>",
		"",
		"Отправьте задачи списком (каждая с новой строки) — они добавятся в план на сегодня.",
		"",
		"Команды:",
		"/today — план на сегодня (с кнопками выполнения)",
		"/add <i>текст</i> — добавить задачу",
		"/yesterday — вчерашний план",
		"/week — план на неделю",
		"/id — узнать свой Telegram ID",
		"/help — помощь",
	}, "\n")
	if user.Role == domain.RoleBoss {
		common += "\n\n<b>Для начальника:</b>\n" +
			"/all — задачи всех сотрудников на сегодня\n" +
			"/all вчера — за вчера\n" +
			"/all 2006-01-02 — за конкретную дату\n" +
			"/pending — кто ещё не отписался сегодня\n" +
			"/export — отчёт в CSV (сегодня)\n" +
			"/export неделя — отчёт за неделю\n" +
			"/export 2006-01-02 2006-01-07 — за период\n" +
			"/team — список сотрудников"
	}
	return common
}
