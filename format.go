package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

type SuccessResponse struct {
	Text      string
	DeleteURL string
}

func FormatSuccessResponse(rawJSON []byte) (SuccessResponse, error) {
	trimmed := bytes.TrimSpace(rawJSON)
	var body struct {
		URL       string `json:"url"`
		DeleteURL string `json:"deleteUrl"`
		MIME      string `json:"mime"`
		Size      int64  `json:"size"`
	}
	if err := json.Unmarshal(trimmed, &body); err != nil {
		return SuccessResponse{}, fmt.Errorf("parse jiral response: %w", err)
	}
	if body.URL == "" {
		return SuccessResponse{}, fmt.Errorf("jiral response missing url")
	}

	displayed := struct {
		MIME string `json:"mime"`
		Size int64  `json:"size"`
	}{
		MIME: body.MIME,
		Size: body.Size,
	}
	pretty, err := json.MarshalIndent(displayed, "", "  ")
	if err != nil {
		return SuccessResponse{}, fmt.Errorf("format jiral response: %w", err)
	}

	text := html.EscapeString(body.URL) + "\n" +
		`<pre><code class="language-json">` + html.EscapeString(strings.TrimSpace(string(pretty))) + `</code></pre>`
	return SuccessResponse{Text: text, DeleteURL: body.DeleteURL}, nil
}
