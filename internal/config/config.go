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
	Token        string
	BossIDs      map[int64]bool
	Location     *time.Location
	DBPath       string
	ReminderHour int
	ReminderMin  int
	FollowupHour int
	FollowupMin  int
}

// Load читает конфигурацию из окружения.
//
//	BOT_TOKEN      — токен бота от @BotFather (обязательно)
//	BOSS_IDS       — Telegram ID начальников через запятую (напр. "123,456")
//	BOT_TZ         — таймзона (по умолчанию Europe/Moscow)
//	DB_PATH        — путь к файлу БД (по умолчанию tasks.db)
//	REMINDER_TIME  — время утреннего напоминания HH:MM (по умолчанию 10:00)
//	FOLLOWUP_TIME  — время напоминания не отписавшимся HH:MM (по умолчанию 14:00)
func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("не задан BOT_TOKEN")
	}

	bossIDs := map[int64]bool{}
	for _, p := range strings.Split(os.Getenv("BOSS_IDS"), ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("неверный BOSS_IDS %q: %w", p, err)
		}
		bossIDs[id] = true
	}

	tz := os.Getenv("BOT_TZ")
	if tz == "" {
		tz = "Europe/Moscow"
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
		Token:        token,
		BossIDs:      bossIDs,
		Location:     loc,
		DBPath:       dbPath,
		ReminderHour: hh,
		ReminderMin:  mm,
		FollowupHour: fh,
		FollowupMin:  fm,
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
