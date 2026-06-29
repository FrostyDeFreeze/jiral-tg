package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := LoadConfig(configFileName)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWithConfig(ctx, cfg, log.Default())
}

func runWithConfig(ctx context.Context, cfg Config, logger *log.Logger) error {
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	var app *App
	telegram, err := bot.New(
		cfg.TelegramBotToken,
		bot.WithHTTPClient(time.Minute, httpClient),
		bot.WithWorkers(1),
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			app.HandleUpdate(ctx, b, update)
		}),
	)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	processor := Processor{
		Telegram:                 telegram,
		Jiral:                    NewJiralClient(cfg.JiralBaseURL, cfg.JiralAPIToken, httpClient),
		HTTPClient:               httpClient,
		TelegramMaxDownloadBytes: cfg.TelegramMaxDownloadBytes,
	}
	app = NewApp(cfg, telegram, processor, logger)
	go app.RunWorker(ctx)
	telegram.Start(ctx)
	return nil
}
