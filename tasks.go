package main

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/go-telegram/bot/models"
)

type TaskKind string

const (
	TaskUpload TaskKind = "upload"
	TaskFetch  TaskKind = "fetch"
)

type Task struct {
	Kind      TaskKind
	ChatID    int64
	MessageID int
	FileID    string
	FileName  string
	Size      int64
	MediaType string
	SourceURL string
}

var httpsURLPattern = regexp.MustCompile(`https://[^\s<>"']+`)

func ExtractTasks(msg *models.Message) []Task {
	if msg == nil {
		return nil
	}
	var tasks []Task
	if task, ok := extractAttachmentTask(msg); ok {
		tasks = append(tasks, task)
	}
	for _, text := range []string{msg.Text, msg.Caption} {
		for _, rawURL := range ExtractSupportedURLs(text) {
			tasks = append(tasks, Task{
				Kind:      TaskFetch,
				ChatID:    msg.Chat.ID,
				MessageID: msg.ID,
				SourceURL: rawURL,
			})
		}
	}
	return tasks
}

func extractAttachmentTask(msg *models.Message) (Task, bool) {
	base := Task{Kind: TaskUpload, ChatID: msg.Chat.ID, MessageID: msg.ID}
	if msg.Document != nil {
		base.FileID = msg.Document.FileID
		base.FileName = defaultFileName(msg.Document.FileName, "document.bin")
		base.Size = msg.Document.FileSize
		base.MediaType = "document"
		return base, true
	}
	if len(msg.Photo) > 0 {
		photo := largestPhoto(msg.Photo)
		base.FileID = photo.FileID
		base.FileName = "photo.jpg"
		base.Size = int64(photo.FileSize)
		base.MediaType = "photo"
		return base, true
	}
	if msg.Video != nil {
		base.FileID = msg.Video.FileID
		base.FileName = defaultFileName(msg.Video.FileName, "video.mp4")
		base.Size = msg.Video.FileSize
		base.MediaType = "video"
		return base, true
	}
	if msg.Animation != nil {
		base.FileID = msg.Animation.FileID
		base.FileName = defaultFileName(msg.Animation.FileName, "animation.mp4")
		base.Size = msg.Animation.FileSize
		base.MediaType = "animation"
		return base, true
	}
	if msg.Audio != nil {
		base.FileID = msg.Audio.FileID
		base.FileName = defaultFileName(msg.Audio.FileName, "audio.mp3")
		base.Size = msg.Audio.FileSize
		base.MediaType = "audio"
		return base, true
	}
	if msg.Voice != nil {
		base.FileID = msg.Voice.FileID
		base.FileName = "voice.ogg"
		base.Size = msg.Voice.FileSize
		base.MediaType = "voice"
		return base, true
	}
	if msg.VideoNote != nil {
		base.FileID = msg.VideoNote.FileID
		base.FileName = "video_note.mp4"
		base.Size = int64(msg.VideoNote.FileSize)
		base.MediaType = "video_note"
		return base, true
	}
	return Task{}, false
}

func largestPhoto(photos []models.PhotoSize) models.PhotoSize {
	best := photos[0]
	bestScore := photoScore(best)
	for _, photo := range photos[1:] {
		if score := photoScore(photo); score > bestScore {
			best = photo
			bestScore = score
		}
	}
	return best
}

func photoScore(photo models.PhotoSize) int64 {
	if photo.FileSize > 0 {
		return int64(photo.FileSize)
	}
	return int64(photo.Width) * int64(photo.Height)
}

func defaultFileName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ExtractSupportedURLs(text string) []string {
	if text == "" {
		return nil
	}
	matches := httpsURLPattern.FindAllString(text, -1)
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		candidate := strings.TrimRight(match, ".,;:!?)]")
		if isSupportedRemoteURL(candidate) {
			urls = append(urls, candidate)
		}
	}
	return urls
}

func isSupportedRemoteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	return host == "instagram.com" || strings.HasSuffix(host, ".instagram.com") ||
		host == "tiktok.com" || strings.HasSuffix(host, ".tiktok.com")
}
