// Package config читает настройки бота из переменных окружения.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — настройки бота, читаются из переменных окружения.
type Config struct {
	Token         string
	BossIDs       map[int64]bool  // начальники по числовому Telegram ID
	BossUsernames map[string]bool // начальники по username (без @, в нижнем регистре)
	Location      *time.Location
	DBPath        string
	ReminderHour  int
	ReminderMin   int
	FollowupHour  int
	FollowupMin   int
}

// IsBoss сообщает, является ли пользователь начальником — по ID или по username.
// ID надёжнее (не меняется), username — удобный псевдоним; достаточно совпадения
// по любому из них.
func (c *Config) IsBoss(tgID int64, username string) bool {
	if c.BossIDs[tgID] {
		return true
	}
	u := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	return u != "" && c.BossUsernames[u]
}

// Load читает конфигурацию из окружения.
//
//	BOT_TOKEN      — токен бота от @BotFather (обязательно)
//	BOSS_IDS       — начальники через запятую: числовые Telegram ID и/или
//	                 @username (напр. "123,@ivan,olga")
//	BOT_TZ         — таймзона (по умолчанию Asia/Almaty)
//	DB_PATH        — путь к файлу БД (по умолчанию tasks.db)
//	REMINDER_TIME  — время утреннего напоминания HH:MM (по умолчанию 10:00)
//	FOLLOWUP_TIME  — время напоминания не отписавшимся HH:MM (по умолчанию 14:00)
func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("не задан BOT_TOKEN")
	}

	// BOSS_IDS принимает и числовые ID, и @username. Числа — это ID,
	// всё остальное трактуется как username (нормализуется без @ и в нижнем регистре).
	bossIDs := map[int64]bool{}
	bossUsernames := map[string]bool{}
	for _, p := range strings.Split(os.Getenv("BOSS_IDS"), ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if id, err := strconv.ParseInt(p, 10, 64); err == nil {
			bossIDs[id] = true
			continue
		}
		bossUsernames[strings.ToLower(strings.TrimPrefix(p, "@"))] = true
	}

	tz := os.Getenv("BOT_TZ")
	if tz == "" {
		tz = "Asia/Almaty"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("неверная таймзона %q: %w", tz, err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "tasks.db"
	}

	hh, mm := parseHM(os.Getenv("REMINDER_TIME"), 10, 0)
	fh, fm := parseHM(os.Getenv("FOLLOWUP_TIME"), 14, 0)

	return &Config{
		Token:         token,
		BossIDs:       bossIDs,
		BossUsernames: bossUsernames,
		Location:      loc,
		DBPath:        dbPath,
		ReminderHour:  hh,
		ReminderMin:   mm,
		FollowupHour:  fh,
		FollowupMin:   fm,
	}, nil
}

// parseHM разбирает строку "HH:MM"; при ошибке возвращает значения по умолчанию.
func parseHM(s string, defH, defM int) (int, int) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return defH, defM
	}
	h, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return defH, defM
	}
	return h, m
}
