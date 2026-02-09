package gist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAPIBase   = "https://api.github.com"
	defaultUserAgent = "clash-speedtest"
)

type Uploader struct {
	client    *http.Client
	apiBase   string
	userAgent string
}

type updateRequest struct {
	Files map[string]gistFile `json:"files"`
}

type gistFile struct {
	Content string `json:"content"`
}

func NewUploader(client *http.Client) *Uploader {
	return NewUploaderWithBase(client, defaultAPIBase)
}

func NewUploaderWithBase(client *http.Client, apiBase string) *Uploader {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	base := strings.TrimRight(apiBase, "/")
	if base == "" {
		base = defaultAPIBase
	}
	return &Uploader{
		client:    client,
		apiBase:   base,
		userAgent: defaultUserAgent,
	}
}

func ParseGistID(address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", fmt.Errorf("gist address is empty")
	}

	candidate := trimmed
	if strings.Contains(candidate, "gist.github.com") {
		if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
			candidate = "https://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil {
			return "", fmt.Errorf("parse gist address %q failed: %w", address, err)
		}
		path := strings.Trim(parsed.Path, "/")
		if path == "" {
			return "", fmt.Errorf("gist address %q missing gist id", address)
		}
		parts := strings.Split(path, "/")
		if len(parts) == 1 {
			if !isLikelyGistID(parts[0]) {
				return "", fmt.Errorf("gist address %q missing gist id", address)
			}
			return strings.TrimSuffix(parts[0], ".git"), nil
		}
		gistID := strings.TrimSuffix(parts[len(parts)-1], ".git")
		if gistID == "" {
			return "", fmt.Errorf("gist address %q missing gist id", address)
		}
		return gistID, nil
	}

	if strings.Contains(candidate, "/") {
		gistID := lastPathSegment(candidate)
		if gistID == "" {
			return "", fmt.Errorf("gist address %q missing gist id", address)
		}
		return gistID, nil
	}

	return candidate, nil
}

func (u *Uploader) UpdateFile(token, address, filename string, content []byte) error {
	if token == "" {
		return fmt.Errorf("gist token is empty")
	}
	if filename == "" {
		return fmt.Errorf("gist filename is empty")
	}

	gistID, err := ParseGistID(address)
	if err != nil {
		return err
	}

	payload := updateRequest{
		Files: map[string]gistFile{
			filename: {Content: string(content)},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("build gist payload for %s failed: %w", gistID, err)
	}

	request, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/gists/%s", u.apiBase, gistID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request for gist %s failed: %w", gistID, err)
	}
	request.Header.Set("Authorization", "token "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", u.userAgent)

	resp, err := u.client.Do(request)
	if err != nil {
		return fmt.Errorf("update gist %s request failed: %w", gistID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody := readResponseBody(resp.Body)
		return fmt.Errorf("update gist %s failed: status %s, body: %s", gistID, resp.Status, responseBody)
	}

	return nil
}

func lastPathSegment(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}

func readResponseBody(reader io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(reader, 2048))
	if err != nil {
		return fmt.Sprintf("read response body failed: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "<empty>"
	}
	return trimmed
}

func isLikelyGistID(value string) bool {
	if len(value) < 8 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}
