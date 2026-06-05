package storage

import (
	"strings"
	"testing"

	"tasktracker/internal/domain"
	"tasktracker/internal/report"
)

func TestReportAndPending(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// два сотрудника + начальник
	s.UpsertUser(domain.User{TgID: 1, ChatID: 1, FullName: "Иван", Username: "ivan", Role: domain.RoleEmployee})
	s.UpsertUser(domain.User{TgID: 2, ChatID: 2, FullName: "Пётр", Username: "petr", Role: domain.RoleEmployee})
	s.UpsertUser(domain.User{TgID: 9, ChatID: 9, FullName: "Босс", Role: domain.RoleBoss})

	// у Ивана есть задача на сегодня, у Петра — нет
	s.AddTask(1, "Сделать отчёт", "2026-06-04")
	id, _ := s.AddTask(1, "Созвон", "2026-06-04")
	s.ToggleTask(id, 1) // отметили выполненной

	// не отписавшиеся
	missing, err := s.EmployeesWithoutTasks("2026-06-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0].FullName != "Пётр" {
		t.Fatalf("ожидался 1 не отписавшийся (Пётр), получили: %+v", missing)
	}

	// отчёт
	rows, err := s.ReportRows("2026-06-04", "2026-06-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("ожидалось 2 строки отчёта, получили %d", len(rows))
	}
	csv, err := report.BuildCSV(rows)
	if err != nil {
		t.Fatal(err)
	}
	out := string(csv)
	if !strings.Contains(out, "Иван") || !strings.Contains(out, "выполнено") || !strings.Contains(out, "@ivan") {
		t.Fatalf("CSV выглядит некорректно:\n%s", out)
	}
	t.Logf("CSV:\n%s", out)
}
