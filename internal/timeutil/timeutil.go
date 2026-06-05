// Package timeutil содержит помощники по работе с датами и их русской локализацией.
package timeutil

import (
	"fmt"
	"time"
)

var ruWeekday = map[time.Weekday]string{
	time.Monday:    "Пн",
	time.Tuesday:   "Вт",
	time.Wednesday: "Ср",
	time.Thursday:  "Чт",
	time.Friday:    "Пт",
	time.Saturday:  "Сб",
	time.Sunday:    "Вс",
}

// DayStr форматирует дату как YYYY-MM-DD.
func DayStr(t time.Time) string { return t.Format("2006-01-02") }

// RuDate превращает "2006-01-02" в "Пн 02.01".
func RuDate(dateStr string, loc *time.Location) string {
	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return dateStr
	}
	return fmt.Sprintf("%s %s", ruWeekday[t.Weekday()], t.Format("02.01"))
}

// WeekRange возвращает понедельник и воскресенье текущей недели (как YYYY-MM-DD).
func WeekRange(now time.Time) (from, to string) {
	wd := int(now.Weekday())
	if wd == 0 { // воскресенье в Go = 0, считаем как 7
		wd = 7
	}
	monday := now.AddDate(0, 0, -(wd - 1))
	sunday := monday.AddDate(0, 0, 6)
	return DayStr(monday), DayStr(sunday)
}

// Short безопасно (по рунам) обрезает строку до n символов.
func Short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
