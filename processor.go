package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var errTelegramFileTooLarge = errors.New("telegram file is larger than configured limit")

type TelegramAPI interface {
	GetFile(context.Context, *bot.GetFileParams) (*models.File, error)
	SendMessage(context.Context, *bot.SendMessageParams) (*models.Message, error)
	FileDownloadLink(*models.File) string
}

type Processor struct {
	Telegram                 TelegramAPI
	Jiral                    *JiralClient
	HTTPClient               *http.Client
	TelegramMaxDownloadBytes int64
}

func (p Processor) Process(ctx context.Context, task Task) (JiralResponse, error) {
	switch task.Kind {
	case TaskFetch:
		return p.Jiral.Fetch(ctx, task.SourceURL)
	case TaskUpload:
		return p.processUpload(ctx, task)
	default:
		return JiralResponse{}, fmt.Errorf("unsupported task kind %q", task.Kind)
	}
}

func (p Processor) processUpload(ctx context.Context, task Task) (JiralResponse, error) {
	if err := checkTelegramFileSize(task.Size, p.TelegramMaxDownloadBytes); err != nil {
		return JiralResponse{}, err
	}
	file, err := p.Telegram.GetFile(ctx, &bot.GetFileParams{FileID: task.FileID})
	if err != nil {
		return JiralResponse{}, fmt.Errorf("telegram getFile: %w", err)
	}
	if file == nil || file.FilePath == "" {
		return JiralResponse{}, fmt.Errorf("telegram getFile returned empty file path")
	}
	if err := checkTelegramFileSize(file.FileSize, p.TelegramMaxDownloadBytes); err != nil {
		return JiralResponse{}, err
	}

	httpClient := p.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.Telegram.FileDownloadLink(file), nil)
	if err != nil {
		return JiralResponse{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return JiralResponse{}, fmt.Errorf("telegram download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return JiralResponse{}, fmt.Errorf("telegram download %s", resp.Status)
	}
	return p.Jiral.Upload(ctx, task.FileName, resp.Body)
}

func checkTelegramFileSize(size, max int64) error {
	if size > 0 && max > 0 && size > max {
		return fmt.Errorf("%w: %d bytes exceeds %d bytes", errTelegramFileTooLarge, size, max)
	}
	return nil
}
