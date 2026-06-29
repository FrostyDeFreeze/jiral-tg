package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

type JiralResponse struct {
	RawJSON []byte
}

type JiralClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

func NewJiralClient(baseURL, apiToken string, httpClient *http.Client) *JiralClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &JiralClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiToken:   apiToken,
		httpClient: httpClient,
	}
}

func (c *JiralClient) Upload(ctx context.Context, fileName string, content io.Reader) (JiralResponse, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, content); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/files", pr)
	if err != nil {
		return JiralResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return c.do(req)
}

func (c *JiralClient) Fetch(ctx context.Context, sourceURL string) (JiralResponse, error) {
	body, err := json.Marshal(map[string]string{"url": sourceURL})
	if err != nil {
		return JiralResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/files/fetch", bytes.NewReader(body))
	if err != nil {
		return JiralResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *JiralClient) do(req *http.Request) (JiralResponse, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return JiralResponse{}, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return JiralResponse{}, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		message := strings.TrimSpace(string(raw))
		if message == "" {
			message = resp.Status
		}
		return JiralResponse{}, fmt.Errorf("jiral %s: %s", resp.Status, message)
	}
	return JiralResponse{RawJSON: raw}, nil
}
