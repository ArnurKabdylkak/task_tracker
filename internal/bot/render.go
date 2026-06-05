package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/domain"
	"tasktracker/internal/timeutil"
)

// Этот файл — слой представления: построение текста сообщений и клавиатур.
// Здесь нет обращений к БД и Telegram API, только форматирование.

// splitLines делит текст по '\n' (вынесено для переиспользования в sender).
func splitLines(s string) []string { return strings.Split(s, "\n") }

// buildTaskList строит текст списка задач с чекбоксами и счётчиком выполненных.
func buildTaskList(header string, tasks []domain.Task) string {
	if len(tasks) == 0 {
		return header + "\n\nЗадач нет."
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	done := 0
	for i, t := range tasks {
		box := "⬜"
		if t.Done() {
			box = "✅"
			done++
		}
		fmt.Fprintf(&b, "%d. %s %s\n", i+1, box, html.EscapeString(t.Text))
	}
	fmt.Fprintf(&b, "\nВыполнено: %d из %d", done, len(tasks))
	return b.String()
}

// todayKeyboard строит клавиатуру управления задачами на сегодня (по кнопке на задачу).
func todayKeyboard(tasks []domain.Task) *tgbotapi.InlineKeyboardMarkup {
	if len(tasks) == 0 {
		return nil
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, t := range tasks {
		mark := "✅"
		if t.Done() {
			mark = "↩️" // вернуть в работу
		}
		toggle := tgbotapi.NewInlineKeyboardButtonData(
			mark+" "+timeutil.Short(t.Text, 20), "toggle:"+strconv.FormatInt(t.ID, 10))
		del := tgbotapi.NewInlineKeyboardButtonData("🗑", "del:"+strconv.FormatInt(t.ID, 10))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(toggle, del))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

// userLabel форматирует имя пользователя с @username (если есть).
func userLabel(u domain.User) string {
	name := html.EscapeString(u.FullName)
	if u.Username != "" {
		name += " (@" + html.EscapeString(u.Username) + ")"
	}
	return name
}

// renderWeek строит текст плана на неделю, сгруппированный по дням.
func renderWeek(from, to string, tasks []domain.Task, loc Loc) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📆 План на неделю (%s — %s)\n",
		timeutil.RuDate(from, loc), timeutil.RuDate(to, loc))

	if len(tasks) == 0 {
		b.WriteString("\nНа этой неделе задач нет.")
		return b.String()
	}

	byDate := map[string][]domain.Task{}
	var order []string
	for _, t := range tasks {
		if _, ok := byDate[t.TaskDate]; !ok {
			order = append(order, t.TaskDate)
		}
		byDate[t.TaskDate] = append(byDate[t.TaskDate], t)
	}
	for _, d := range order {
		fmt.Fprintf(&b, "\n<b>%s</b>\n", timeutil.RuDate(d, loc))
		for _, t := range byDate[d] {
			box := "⬜"
			if t.Done() {
				box = "✅"
			}
			fmt.Fprintf(&b, "  %s %s\n", box, html.EscapeString(t.Text))
		}
	}
	return b.String()
}

// pendingList форматирует список не отписавшихся сотрудников.
func pendingList(missing []domain.User) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⚠️ Не отписались сегодня (%d):\n", len(missing))
	for i, e := range missing {
		fmt.Fprintf(&b, "%d. %s\n", i+1, userLabel(e))
	}
	return b.String()
}

// teamList форматирует список сотрудников.
func teamList(emps []domain.User) string {
	var b strings.Builder
	fmt.Fprintf(&b, "👥 Сотрудников: %d\n", len(emps))
	for i, e := range emps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, userLabel(e))
	}
	return b.String()
}
