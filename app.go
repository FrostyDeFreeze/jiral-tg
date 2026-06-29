package main

import (
	"context"
	"errors"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	unsupportedMessageText = "Send a file or an Instagram/TikTok link"
	queueFullMessage       = "Upload queue is full, try again later"
)

type App struct {
	cfg       Config
	telegram  TelegramAPI
	processor Processor
	queue     chan Task
	logger    *log.Logger
}

func NewApp(cfg Config, telegram TelegramAPI, processor Processor, logger *log.Logger) *App {
	if logger == nil {
		logger = log.Default()
	}
	return &App{
		cfg:       cfg,
		telegram:  telegram,
		processor: processor,
		queue:     make(chan Task, cfg.QueueSize),
		logger:    logger,
	}
}

func (a *App) HandleUpdate(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		return
	}
	msg := update.Message
	userID, ok := messageUserID(msg)
	if !ok || !a.cfg.AllowsUser(userID) {
		a.logger.Printf("unauthorized user_id=%d chat_id=%d update_id=%d type=message", userID, msg.Chat.ID, update.ID)
		return
	}

	tasks := ExtractTasks(msg)
	if len(tasks) == 0 {
		a.sendText(ctx, msg.Chat.ID, unsupportedMessageText)
		return
	}
	for _, task := range tasks {
		select {
		case a.queue <- task:
		default:
			a.sendText(ctx, task.ChatID, queueFullMessage)
		}
	}
}

func (a *App) RunWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-a.queue:
			a.runTask(ctx, task)
		}
	}
}

func (a *App) runTask(ctx context.Context, task Task) {
	resp, err := a.processor.Process(ctx, task)
	if err != nil {
		a.logger.Printf("task_failed kind=%s chat_id=%d message_id=%d err=%q", task.Kind, task.ChatID, task.MessageID, err)
		a.sendText(ctx, task.ChatID, formatProcessingError(err))
		return
	}
	success, err := FormatSuccessResponse(resp.RawJSON)
	if err != nil {
		a.logger.Printf("format_response_failed kind=%s chat_id=%d message_id=%d err=%q", task.Kind, task.ChatID, task.MessageID, err)
		a.sendText(ctx, task.ChatID, formatResponseErrorMessage(err))
		return
	}
	a.sendSuccess(ctx, task.ChatID, success)
}

func (a *App) sendText(ctx context.Context, chatID int64, text string) {
	a.sendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

func (a *App) sendSuccess(ctx context.Context, chatID int64, success SuccessResponse) {
	params := &bot.SendMessageParams{ChatID: chatID, Text: success.Text, ParseMode: models.ParseModeHTML}
	if success.DeleteURL != "" {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "🗑️", URL: success.DeleteURL},
			}},
		}
	}
	a.sendMessage(ctx, params)
}

func (a *App) sendMessage(ctx context.Context, params *bot.SendMessageParams) {
	if _, err := a.telegram.SendMessage(ctx, params); err != nil {
		a.logger.Printf("send_message_failed chat_id=%v err=%q", params.ChatID, err)
	}
}

func messageUserID(msg *models.Message) (int64, bool) {
	if msg.From == nil {
		return 0, false
	}
	return msg.From.ID, true
}

func formatResponseErrorMessage(err error) string {
	return "File uploaded, but the server response could not be formatted: " + err.Error()
}

func formatProcessingError(err error) string {
	if errors.Is(err, errTelegramFileTooLarge) {
		return "File is larger than the Telegram Bot API limit: " + err.Error()
	}
	return "Upload failed: " + err.Error()
}
