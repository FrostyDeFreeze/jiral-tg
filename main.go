package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	// the cloud Bot API does not let bots download bigger files
	telegramFileLimit = 20 << 20
	httpTimeout       = 3 * time.Minute
)

// links from these hosts and their subdomains go to jiral fetch (yt-dlp)
var fetchHosts = []string{
	"instagram.com", "tiktok.com",
	"twitch.tv", "youtube.com", "youtu.be",
	"x.com", "twitter.com",
}

type config struct {
	TelegramBotToken string  `json:"telegramBotToken"`
	AllowedUserIDs   []int64 `json:"allowedUserIds"`
	JiralBaseURL     string  `json:"jiralBaseUrl"`
	JiralAPIToken    string  `json:"jiralApiToken"`
}

type app struct {
	config
	http *http.Client
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := loadConfig("config.json")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	a := &app{config: cfg, http: &http.Client{Timeout: httpTimeout}}
	b, err := bot.New(cfg.TelegramBotToken,
		bot.WithHTTPClient(time.Minute, a.http),
		// one message at a time: uploads and fetches never run in parallel
		bot.WithWorkers(1),
		bot.WithNotAsyncHandlers(),
		bot.WithDefaultHandler(a.handle),
		bot.WithErrorsHandler(func(err error) { slog.Error("telegram", "err", err) }),
	)
	if err != nil {
		slog.Error("bot", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("polling")
	b.Start(ctx)
}

func loadConfig(path string) (config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	var cfg config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.TelegramBotToken == "" || len(cfg.AllowedUserIDs) == 0 || cfg.JiralBaseURL == "" || cfg.JiralAPIToken == "" {
		return config{}, errors.New("config needs telegramBotToken, allowedUserIds, jiralBaseUrl and jiralApiToken")
	}

	cfg.JiralBaseURL = strings.TrimRight(cfg.JiralBaseURL, "/")

	return cfg, nil
}

// handle uploads the message's attachment and fetches every supported link in it.
// An album arrives as one message per file, so each file gets its own reply
func (a *app) handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	if msg.From == nil || !slices.Contains(a.AllowedUserIDs, msg.From.ID) {
		slog.Warn("unauthorized", "chat", msg.Chat.ID)
		return
	}

	links := fetchLinks(msg.Text + " " + msg.Caption)
	id, name, size := attachment(msg)
	if id == "" && len(links) == 0 {
		reply(ctx, b, msg, "Send a file or a link from Instagram, TikTok, Twitch, YouTube or X", nil)
		return
	}

	if id != "" {
		f, err := a.upload(ctx, b, id, name, size)
		respond(ctx, b, msg, name, f, err)
	}

	for _, link := range links {
		f, err := a.fetch(ctx, link)
		respond(ctx, b, msg, link, f, err)
	}
}

// attachment returns the file of any kind, including files sent as documents.
// The name gives jiral the extension; a ".bin" fallback makes it sniff the content
func attachment(m *models.Message) (id, name string, size int64) {
	switch {
	case m.Document != nil:
		return m.Document.FileID, or(m.Document.FileName, "document.bin"), m.Document.FileSize
	case len(m.Photo) > 0:
		p := m.Photo[len(m.Photo)-1] // sizes go from smallest to largest
		return p.FileID, "photo.jpg", int64(p.FileSize)
	case m.Video != nil:
		return m.Video.FileID, or(m.Video.FileName, "video.mp4"), m.Video.FileSize
	case m.Animation != nil:
		return m.Animation.FileID, or(m.Animation.FileName, "animation.mp4"), m.Animation.FileSize
	case m.Audio != nil:
		return m.Audio.FileID, or(m.Audio.FileName, "audio.mp3"), m.Audio.FileSize
	case m.Voice != nil:
		return m.Voice.FileID, "voice.ogg", m.Voice.FileSize
	case m.VideoNote != nil:
		return m.VideoNote.FileID, "video_note.mp4", int64(m.VideoNote.FileSize)
	}

	return "", "", 0
}

func or(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}

	return fallback
}

// shortcut: only links written out in the text, not hidden under text (text_link entities)
func fetchLinks(text string) []string {
	var links []string
	for _, word := range strings.Fields(text) {
		raw := strings.TrimRight(word, ".,;:!?)]")
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.User != nil {
			continue
		}

		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		if slices.ContainsFunc(fetchHosts, func(h string) bool { return host == h || strings.HasSuffix(host, "."+h) }) {
			links = append(links, raw)
		}
	}

	return links
}

// respond logs the result and replies with the file link, its type and size, or with the error
func respond(ctx context.Context, b *bot.Bot, msg *models.Message, source string, f file, err error) {
	if err != nil {
		slog.Error("upload failed", "source", source, "err", err)
		reply(ctx, b, msg, "<b>Upload failed</b>\n<blockquote>"+html.EscapeString(err.Error())+"</blockquote>", nil)
		return
	}

	slog.Info("uploaded", "source", source, "name", f.Name, "mime", f.MIME, "size", f.Size)
	text := `<a href="` + html.EscapeString(f.URL) + `">` + html.EscapeString(f.Name) + "</a>\n" +
		"<code>" + html.EscapeString(f.MIME) + " · " + humanSize(f.Size) + "</code>"
	buttons := []models.InlineKeyboardButton{
		{Text: "📋", CopyText: &models.CopyTextButton{Text: f.URL}},
		{Text: "🗑️", URL: f.DeleteURL},
	}

	reply(ctx, b, msg, text, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{buttons}})
}

func reply(ctx context.Context, b *bot.Bot, msg *models.Message, text string, markup models.ReplyMarkup) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.ID, AllowSendingWithoutReply: true},
		ReplyMarkup:     markup,
	})
	if err != nil {
		slog.Error("send failed", "chat", msg.Chat.ID, "err", err)
	}
}

// humanSize matches jiral's og:description, e.g. "4.2 MB"
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	div := int64(unit)
	exp := 0
	for n := size / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}
