// Package storage реализует domain.Repository поверх SQLite.
package storage

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"tasktracker/internal/domain"
)

// Store — обёртка над базой данных, реализующая domain.Repository.
type Store struct {
	db *sql.DB
}

// New открывает (или создаёт) базу и инициализирует схему.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
    tg_id      INTEGER PRIMARY KEY,
    chat_id    INTEGER NOT NULL,
    username   TEXT,
    full_name  TEXT,
    role       TEXT NOT NULL DEFAULT 'employee',
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tasks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL,
    text       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    task_date  TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_user_date ON tasks(user_id, task_date);
`
	_, err := s.db.Exec(schema)
	return err
}

// Close закрывает соединение с БД.
func (s *Store) Close() error { return s.db.Close() }

// UpsertUser создаёт пользователя или обновляет его данные (включая роль).
func (s *Store) UpsertUser(u domain.User) error {
	_, err := s.db.Exec(`
INSERT INTO users (tg_id, chat_id, username, full_name, role, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(tg_id) DO UPDATE SET
    chat_id   = excluded.chat_id,
    username  = excluded.username,
    full_name = excluded.full_name,
    role      = excluded.role`,
		u.TgID, u.ChatID, u.Username, u.FullName, string(u.Role),
		time.Now().Format(time.RFC3339))
	return err
}

// GetUser возвращает пользователя по Telegram ID (nil, nil если не найден).
func (s *Store) GetUser(tgID int64) (*domain.User, error) {
	row := s.db.QueryRow(
		`SELECT tg_id, chat_id, username, full_name, role FROM users WHERE tg_id = ?`, tgID)
	var u domain.User
	err := row.Scan(&u.TgID, &u.ChatID, &u.Username, &u.FullName, &u.Role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUsersByRole возвращает всех пользователей с указанной ролью.
func (s *Store) GetUsersByRole(role domain.Role) ([]domain.User, error) {
	rows, err := s.db.Query(
		`SELECT tg_id, chat_id, username, full_name, role FROM users WHERE role = ? ORDER BY full_name`,
		string(role))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.TgID, &u.ChatID, &u.Username, &u.FullName, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AddTask добавляет задачу на указанную дату.
func (s *Store) AddTask(userID int64, text, date string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO tasks (user_id, text, status, task_date, created_at) VALUES (?, ?, 'pending', ?, ?)`,
		userID, text, date, time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TasksByDate возвращает задачи пользователя за конкретный день.
func (s *Store) TasksByDate(userID int64, date string) ([]domain.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, text, status, task_date, created_at
		 FROM tasks WHERE user_id = ? AND task_date = ? ORDER BY id`, userID, date)
	if err != nil {
		return nil, err
	}
	return scanTasks(rows)
}

// TasksBetween возвращает задачи пользователя за диапазон дат (включительно).
func (s *Store) TasksBetween(userID int64, from, to string) ([]domain.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, text, status, task_date, created_at
		 FROM tasks WHERE user_id = ? AND task_date BETWEEN ? AND ?
		 ORDER BY task_date, id`, userID, from, to)
	if err != nil {
		return nil, err
	}
	return scanTasks(rows)
}

// ToggleTask переключает статус задачи (только для своих задач).
func (s *Store) ToggleTask(id, userID int64) error {
	_, err := s.db.Exec(
		`UPDATE tasks SET status = CASE WHEN status='done' THEN 'pending' ELSE 'done' END
		 WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// DeleteTask удаляет задачу (только свою).
func (s *Store) DeleteTask(id, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ReportRows возвращает все задачи всех пользователей за диапазон дат.
func (s *Store) ReportRows(from, to string) ([]domain.ReportRow, error) {
	rows, err := s.db.Query(`
SELECT t.task_date, u.full_name, COALESCE(u.username, ''), t.text, t.status
FROM tasks t
JOIN users u ON u.tg_id = t.user_id
WHERE t.task_date BETWEEN ? AND ?
ORDER BY t.task_date, u.full_name, t.id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ReportRow
	for rows.Next() {
		var r domain.ReportRow
		if err := rows.Scan(&r.Date, &r.FullName, &r.Username, &r.Text, &r.Status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EmployeesWithoutTasks возвращает сотрудников, у которых нет задач на указанную дату.
func (s *Store) EmployeesWithoutTasks(date string) ([]domain.User, error) {
	rows, err := s.db.Query(`
SELECT u.tg_id, u.chat_id, u.username, u.full_name, u.role
FROM users u
WHERE u.role = 'employee'
  AND NOT EXISTS (
        SELECT 1 FROM tasks t WHERE t.user_id = u.tg_id AND t.task_date = ?
  )
ORDER BY u.full_name`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.TgID, &u.ChatID, &u.Username, &u.FullName, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func scanTasks(rows *sql.Rows) ([]domain.Task, error) {
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.UserID, &t.Text, &t.Status, &t.TaskDate, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
