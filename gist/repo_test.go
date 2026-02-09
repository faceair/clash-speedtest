package gist

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRepoAddress(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedOwner string
		expectedRepo  string
		wantErr       bool
	}{
		{
			name:          "owner repo",
			input:         "faceair/clash-speedtest",
			expectedOwner: "faceair",
			expectedRepo:  "clash-speedtest",
		},
		{
			name:          "github url",
			input:         "https://github.com/faceair/clash-speedtest",
			expectedOwner: "faceair",
			expectedRepo:  "clash-speedtest",
		},
		{
			name:          "github url without scheme",
			input:         "github.com/faceair/clash-speedtest",
			expectedOwner: "faceair",
			expectedRepo:  "clash-speedtest",
		},
		{
			name:          "git suffix",
			input:         "https://github.com/faceair/clash-speedtest.git",
			expectedOwner: "faceair",
			expectedRepo:  "clash-speedtest",
		},
		{
			name:    "empty",
			input:   "  ",
			wantErr: true,
		},
		{
			name:    "missing repo",
			input:   "faceair",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			owner, repo, err := ParseRepoAddress(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != test.expectedOwner || repo != test.expectedRepo {
				t.Fatalf("expected %s/%s, got %s/%s", test.expectedOwner, test.expectedRepo, owner, repo)
			}
		})
	}
}

func TestUpdateRepoFile(t *testing.T) {
	t.Run("update existing file", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestCount++
			if request.URL.Path != "/repos/faceair/clash-speedtest/contents/configs/result.yaml" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			if request.Header.Get("Authorization") != "token test-token" {
				t.Fatalf("unexpected auth header: %s", request.Header.Get("Authorization"))
			}
			if request.Header.Get("User-Agent") != defaultUserAgent {
				t.Fatalf("unexpected user agent: %s", request.Header.Get("User-Agent"))
			}

			switch request.Method {
			case http.MethodGet:
				if request.URL.Query().Get("ref") != "main" {
					t.Fatalf("unexpected ref query: %s", request.URL.RawQuery)
				}
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(`{"sha":"abc123"}`))
			case http.MethodPut:
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read body failed: %v", err)
				}

				var payload repoUpdateRequest
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("unmarshal payload failed: %v", err)
				}
				if payload.SHA != "abc123" {
					t.Fatalf("unexpected sha: %s", payload.SHA)
				}
				if payload.Branch != "main" {
					t.Fatalf("unexpected branch: %s", payload.Branch)
				}
				if payload.Content != base64.StdEncoding.EncodeToString([]byte("payload")) {
					t.Fatalf("unexpected content: %s", payload.Content)
				}
				if !strings.Contains(payload.Message, "configs/result.yaml") {
					t.Fatalf("unexpected message: %s", payload.Message)
				}

				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(`{"content":{"sha":"def456"}}`))
			default:
				t.Fatalf("unexpected method: %s", request.Method)
			}
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		if err := uploader.UpdateRepoFile("test-token", "https://github.com/faceair/clash-speedtest", "configs/result.yaml", "main", []byte("payload")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if requestCount != 2 {
			t.Fatalf("expected 2 requests, got %d", requestCount)
		}
	})

	t.Run("create new file when missing", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestCount++
			switch request.Method {
			case http.MethodGet:
				writer.WriteHeader(http.StatusNotFound)
				_, _ = writer.Write([]byte(`{"message":"Not Found"}`))
			case http.MethodPut:
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read body failed: %v", err)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("unmarshal payload failed: %v", err)
				}
				if _, exists := payload["sha"]; exists {
					t.Fatalf("sha should be omitted for create payload")
				}
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(`{"content":{"sha":"ghi789"}}`))
			default:
				t.Fatalf("unexpected method: %s", request.Method)
			}
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		if err := uploader.UpdateRepoFile("test-token", "faceair/clash-speedtest", "result.yaml", "", []byte("payload")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if requestCount != 2 {
			t.Fatalf("expected 2 requests, got %d", requestCount)
		}
	})

	t.Run("get sha failed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("boom"))
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		err := uploader.UpdateRepoFile("test-token", "faceair/clash-speedtest", "result.yaml", "", []byte("payload"))
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "get repo file") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update failed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.Method {
			case http.MethodGet:
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(`{"sha":"abc123"}`))
			case http.MethodPut:
				writer.WriteHeader(http.StatusConflict)
				_, _ = writer.Write([]byte("conflict"))
			default:
				t.Fatalf("unexpected method: %s", request.Method)
			}
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		err := uploader.UpdateRepoFile("test-token", "faceair/clash-speedtest", "result.yaml", "", []byte("payload"))
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "update repo file") || !strings.Contains(err.Error(), "conflict") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
