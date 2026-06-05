// Package domain содержит модели предметной области и контракты хранилища.
// Пакет не зависит ни от Telegram, ни от конкретной СУБД — это ядро приложения.
package domain

// Role — роль пользователя в системе.
type Role string

const (
	RoleEmployee Role = "employee" // сотрудник
	RoleBoss     Role = "boss"     // начальник
)

// Status — статус задачи.
const (
	StatusPending = "pending"
	StatusDone    = "done"
)

// User — зарегистрированный пользователь бота.
type User struct {
	TgID     int64
	ChatID   int64
	Username string
	FullName string
	Role     Role
}

// Task — одна задача сотрудника на конкретный день.
type Task struct {
	ID        int64
	UserID    int64
	Text      string
	Status    string // pending | done
	TaskDate  string // YYYY-MM-DD
	CreatedAt string
}

// Done сообщает, выполнена ли задача.
func (t Task) Done() bool { return t.Status == StatusDone }

// ReportRow — строка сводного отчёта (задача + автор).
type ReportRow struct {
	Date     string
	FullName string
	Username string
	Text     string
	Status   string
}

// Repository — контракт доступа к данным. Реализуется в пакете storage,
// потребляется хендлерами и планировщиком. Позволяет подменять хранилище
// в тестах и при смене СУБД.
type Repository interface {
	UpsertUser(u User) error
	GetUser(tgID int64) (*User, error)
	GetUsersByRole(role Role) ([]User, error)

	AddTask(userID int64, text, date string) (int64, error)
	TasksByDate(userID int64, date string) ([]Task, error)
	TasksBetween(userID int64, from, to string) ([]Task, error)
	ToggleTask(id, userID int64) error
	DeleteTask(id, userID int64) error

	ReportRows(from, to string) ([]ReportRow, error)
	EmployeesWithoutTasks(date string) ([]User, error)

	Close() error
}
