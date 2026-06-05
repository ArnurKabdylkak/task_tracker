// Command tasktracker — Telegram-бот ежедневной отчётности по задачам.
package main

import (
	"log"
	_ "time/tzdata" // встроенная база таймзон, чтобы LoadLocation работал в любом окружении

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tasktracker/internal/bot"
	"tasktracker/internal/config"
	"tasktracker/internal/scheduler"
	"tasktracker/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("конфигурация: %v", err)
	}

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("база данных: %v", err)
	}
	defer store.Close()

	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Fatalf("телеграм: %v", err)
	}
	log.Printf("Авторизован как @%s", api.Self.UserName)

	setCommands(api)

	h := bot.NewHandler(api, store, cfg)
	scheduler.New(api, store, cfg).Start()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := api.GetUpdatesChan(u)
	for update := range updates {
		h.Handle(update)
	}
}

// setCommands регистрирует меню команд (отображается в клиенте Telegram).
func setCommands(api *tgbotapi.BotAPI) {
	cmds := []tgbotapi.BotCommand{
		{Command: "today", Description: "План на сегодня"},
		{Command: "add", Description: "Добавить задачу"},
		{Command: "yesterday", Description: "Вчерашний план"},
		{Command: "week", Description: "План на неделю"},
		{Command: "all", Description: "Задачи команды (для начальника)"},
		{Command: "pending", Description: "Кто не отписался (для начальника)"},
		{Command: "export", Description: "Отчёт в CSV (для начальника)"},
		{Command: "team", Description: "Список сотрудников (для начальника)"},
		{Command: "id", Description: "Узнать свой Telegram ID"},
		{Command: "help", Description: "Помощь"},
	}
	if _, err := api.Request(tgbotapi.NewSetMyCommands(cmds...)); err != nil {
		log.Printf("setCommands: %v", err)
	}
}
