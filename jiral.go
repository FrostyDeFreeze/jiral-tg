package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/go-telegram/bot"
)

// file is jiral's upload and fetch response
type file struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	DeleteURL string `json:"deleteUrl"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
}

// upload downloads the file from Telegram and sends it to jiral
func (a *app) upload(ctx context.Context, b *bot.Bot, id, name string, size int64) (file, error) {
	if size > telegramFileLimit {
		return file{}, errors.New("Telegram does not let bots download files over 20 MB")
	}

	tf, err := b.GetFile(ctx, &bot.GetFileParams{FileID: id})
	if err != nil {
		return file{}, fmt.Errorf("telegram getFile: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(tf), nil)
	if err != nil {
		return file{}, err
	}

	resp, err := a.http.Do(req)
	if err != nil {
		return file{}, fmt.Errorf("telegram download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return file{}, fmt.Errorf("telegram download: %s", resp.Status)
	}

	// files are at most 20 MB, so the body is built in memory
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", name) // writes to memory, cannot fail
	if _, err := io.Copy(part, resp.Body); err != nil {
		return file{}, fmt.Errorf("telegram download: %w", err)
	}

	w.Close()

	return a.jiral(ctx, "/api/upload", w.FormDataContentType(), &body)
}

// fetch makes jiral download the link with yt-dlp
func (a *app) fetch(ctx context.Context, link string) (file, error) {
	body, _ := json.Marshal(map[string]string{"url": link})

	return a.jiral(ctx, "/api/fetch", "application/json", bytes.NewReader(body))
}

func (a *app) jiral(ctx context.Context, path, contentType string, body io.Reader) (file, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.JiralBaseURL+path, body)
	if err != nil {
		return file{}, err
	}

	req.Header.Set("Authorization", "Bearer "+a.JiralAPIToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := a.http.Do(req)
	if err != nil {
		return file{}, err
	}
	defer resp.Body.Close()

	var out struct {
		file
		Error string `json:"error"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if out.Error == "" {
			return file{}, fmt.Errorf("jiral %s", resp.Status)
		}

		return file{}, fmt.Errorf("jiral %s: %s", resp.Status, out.Error)
	}

	if decodeErr != nil || out.URL == "" {
		return file{}, errors.New("jiral returned an unreadable response")
	}

	return out.file, nil
}
