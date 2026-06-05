// Package report формирует выгрузки отчётов.
package report

import (
	"bytes"
	"encoding/csv"

	"tasktracker/internal/domain"
)

// BuildCSV формирует CSV-отчёт по строкам.
// Используется BOM и разделитель ';' — так Excel в русской локали
// корректно открывает файл с кириллицей.
func BuildCSV(rows []domain.ReportRow) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	if err := w.Write([]string{"Дата", "Сотрудник", "Username", "Задача", "Статус"}); err != nil {
		return nil, err
	}
	for _, r := range rows {
		status := "не выполнено"
		if r.Status == domain.StatusDone {
			status = "выполнено"
		}
		username := ""
		if r.Username != "" {
			username = "@" + r.Username
		}
		if err := w.Write([]string{r.Date, r.FullName, username, r.Text, status}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
