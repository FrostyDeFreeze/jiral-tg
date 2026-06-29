package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	configFileName                  = "config.json"
	defaultTelegramMaxDownloadBytes = int64(20 * 1024 * 1024)
	defaultHTTPTimeoutSeconds       = 180
	defaultQueueSize                = 32
)

type Config struct {
	TelegramBotToken         string        `json:"telegramBotToken"`
	AllowedUserIDs           []int64       `json:"allowedUserIds"`
	JiralBaseURL             string        `json:"jiralBaseUrl"`
	JiralAPIToken            string        `json:"jiralApiToken"`
	TelegramMaxDownloadBytes int64         `json:"telegramMaxDownloadBytes"`
	HTTPTimeoutSeconds       int           `json:"httpTimeoutSeconds"`
	QueueSize                int           `json:"queueSize"`
	HTTPTimeout              time.Duration `json:"-"`
	allowedUserIDLookup      map[int64]struct{}
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg Config) (Config, error) {
	var missing []string
	if strings.TrimSpace(cfg.TelegramBotToken) == "" {
		missing = append(missing, "telegramBotToken")
	}
	if len(cfg.AllowedUserIDs) == 0 {
		missing = append(missing, "allowedUserIds")
	}
	if strings.TrimSpace(cfg.JiralBaseURL) == "" {
		missing = append(missing, "jiralBaseUrl")
	}
	if strings.TrimSpace(cfg.JiralAPIToken) == "" {
		missing = append(missing, "jiralApiToken")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required config fields: %s", strings.Join(missing, ", "))
	}

	cfg.TelegramBotToken = strings.TrimSpace(cfg.TelegramBotToken)
	cfg.JiralBaseURL = strings.TrimRight(strings.TrimSpace(cfg.JiralBaseURL), "/")
	cfg.JiralAPIToken = strings.TrimSpace(cfg.JiralAPIToken)
	if cfg.TelegramMaxDownloadBytes == 0 {
		cfg.TelegramMaxDownloadBytes = defaultTelegramMaxDownloadBytes
	}
	if cfg.TelegramMaxDownloadBytes < 0 {
		return Config{}, fmt.Errorf("telegramMaxDownloadBytes must not be negative")
	}
	if cfg.HTTPTimeoutSeconds == 0 {
		cfg.HTTPTimeoutSeconds = defaultHTTPTimeoutSeconds
	}
	if cfg.HTTPTimeoutSeconds < 1 {
		return Config{}, fmt.Errorf("httpTimeoutSeconds must be positive")
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.QueueSize < 1 {
		return Config{}, fmt.Errorf("queueSize must be positive")
	}
	cfg.HTTPTimeout = time.Duration(cfg.HTTPTimeoutSeconds) * time.Second
	cfg.allowedUserIDLookup = make(map[int64]struct{}, len(cfg.AllowedUserIDs))
	for _, id := range cfg.AllowedUserIDs {
		cfg.allowedUserIDLookup[id] = struct{}{}
	}
	return cfg, nil
}

func (c Config) AllowsUser(userID int64) bool {
	if c.allowedUserIDLookup != nil {
		_, ok := c.allowedUserIDLookup[userID]
		return ok
	}
	for _, id := range c.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}
