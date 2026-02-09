package gist

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type repoContentResponse struct {
	SHA string `json:"sha"`
}

type repoUpdateRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	SHA     string `json:"sha,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

func ParseRepoAddress(address string) (owner string, repo string, err error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", "", fmt.Errorf("repo address is empty")
	}

	candidate := trimmed
	if strings.Contains(candidate, "github.com") {
		if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
			candidate = "https://" + candidate
		}
		parsed, parseErr := url.Parse(candidate)
		if parseErr != nil {
			return "", "", fmt.Errorf("parse repo address %q failed: %w", address, parseErr)
		}
		owner, repo, err = parseRepoPath(parsed.Path, address)
		if err != nil {
			return "", "", err
		}
		return owner, repo, nil
	}

	owner, repo, err = parseRepoPath(candidate, address)
	if err != nil {
		return "", "", err
	}
	return owner, repo, nil
}

func (u *Uploader) UpdateRepoFile(token, address, filePath, branch string, content []byte) error {
	if token == "" {
		return fmt.Errorf("repo token is empty")
	}

	trimmedPath := strings.TrimSpace(strings.TrimPrefix(filePath, "/"))
	if trimmedPath == "" {
		return fmt.Errorf("repo file path is empty")
	}

	owner, repo, err := ParseRepoAddress(address)
	if err != nil {
		return err
	}

	trimmedBranch := strings.TrimSpace(branch)
	sha, err := u.getRepoFileSHA(token, owner, repo, trimmedPath, trimmedBranch)
	if err != nil {
		return err
	}

	payload := repoUpdateRequest{
		Message: fmt.Sprintf("update %s via clash-speedtest", trimmedPath),
		Content: base64.StdEncoding.EncodeToString(content),
		SHA:     sha,
		Branch:  trimmedBranch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("build repo payload for %s/%s/%s failed: %w", owner, repo, trimmedPath, err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", u.apiBase, url.PathEscape(owner), url.PathEscape(repo), encodeRepoPath(trimmedPath))
	request, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request for repo file %s/%s/%s failed: %w", owner, repo, trimmedPath, err)
	}
	request.Header.Set("Authorization", "token "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", u.userAgent)

	resp, err := u.client.Do(request)
	if err != nil {
		return fmt.Errorf("update repo file %s/%s/%s request failed: %w", owner, repo, trimmedPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody := readResponseBody(resp.Body)
		return fmt.Errorf("update repo file %s/%s/%s failed: status %s, body: %s", owner, repo, trimmedPath, resp.Status, responseBody)
	}

	return nil
}

func (u *Uploader) getRepoFileSHA(token, owner, repo, filePath, branch string) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", u.apiBase, url.PathEscape(owner), url.PathEscape(repo), encodeRepoPath(filePath))
	if branch != "" {
		query := url.Values{}
		query.Set("ref", branch)
		endpoint += "?" + query.Encode()
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create request for repo file %s/%s/%s sha failed: %w", owner, repo, filePath, err)
	}
	request.Header.Set("Authorization", "token "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", u.userAgent)

	resp, err := u.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("get repo file %s/%s/%s sha request failed: %w", owner, repo, filePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody := readResponseBody(resp.Body)
		return "", fmt.Errorf("get repo file %s/%s/%s sha failed: status %s, body: %s", owner, repo, filePath, resp.Status, responseBody)
	}

	var response repoContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode repo file %s/%s/%s sha response failed: %w", owner, repo, filePath, err)
	}
	if response.SHA == "" {
		return "", fmt.Errorf("get repo file %s/%s/%s sha failed: response missing sha", owner, repo, filePath)
	}

	return response.SHA, nil
}

func parseRepoPath(path, address string) (owner string, repo string, err error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("repo address %q missing owner/repo", address)
	}

	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(strings.TrimSuffix(parts[1], ".git"))
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("repo address %q missing owner/repo", address)
	}

	return owner, repo, nil
}

func encodeRepoPath(filePath string) string {
	parts := strings.Split(strings.Trim(filePath, "/"), "/")
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		encoded = append(encoded, url.PathEscape(trimmed))
	}
	return strings.Join(encoded, "/")
}
