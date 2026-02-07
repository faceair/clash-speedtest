package gist

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseGistID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "raw id",
			input:    "abc123",
			expected: "abc123",
		},
		{
			name:     "url with user",
			input:    "https://gist.github.com/user/abc123",
			expected: "abc123",
		},
		{
			name:     "url without scheme",
			input:    "gist.github.com/user/abc123",
			expected: "abc123",
		},
		{
			name:     "user slash id",
			input:    "user/abc123",
			expected: "abc123",
		},
		{
			name:     "url with git suffix",
			input:    "https://gist.github.com/user/abc123.git",
			expected: "abc123",
		},
		{
			name:    "empty",
			input:   "  ",
			wantErr: true,
		},
		{
			name:    "missing id",
			input:   "https://gist.github.com/user/",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id, err := ParseGistID(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, id)
			}
		})
	}
}

func TestUpdateFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", request.Method)
			}
			if request.URL.Path != "/gists/abc123" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			if request.Header.Get("Authorization") != "token test-token" {
				t.Fatalf("unexpected auth header: %s", request.Header.Get("Authorization"))
			}
			if request.Header.Get("User-Agent") != defaultUserAgent {
				t.Fatalf("unexpected user agent: %s", request.Header.Get("User-Agent"))
			}

			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read body failed: %v", err)
			}
			var payload updateRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal payload failed: %v", err)
			}
			file, ok := payload.Files["fastsub.yaml"]
			if !ok {
				t.Fatalf("missing fastsub.yaml payload")
			}
			if file.Content != "payload" {
				t.Fatalf("unexpected content: %s", file.Content)
			}

			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"id":"abc123"}`))
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		if err := uploader.UpdateFile("test-token", "https://gist.github.com/user/abc123", "fastsub.yaml", []byte("payload")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("boom"))
		}))
		defer server.Close()

		uploader := NewUploaderWithBase(server.Client(), server.URL)
		err := uploader.UpdateFile("test-token", "abc123", "fastsub.yaml", []byte("payload"))
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "status") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSetProxy(t *testing.T) {
	t.Run("invalid proxy url", func(t *testing.T) {
		uploader := NewUploader(nil)
		err := uploader.SetProxy("127.0.0.1:7890")
		if err == nil {
			t.Fatalf("expected error for invalid proxy url")
		}
		if !strings.Contains(err.Error(), "parse HTTPS proxy") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("set dedicated https proxy", func(t *testing.T) {
		uploader := NewUploader(nil)
		if err := uploader.SetProxy("http://127.0.0.1:7890"); err != nil {
			t.Fatalf("set proxy failed: %v", err)
		}

		transport, ok := uploader.client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("unexpected transport type: %T", uploader.client.Transport)
		}
		if transport.Proxy == nil {
			t.Fatalf("proxy function was not configured")
		}

		resolved, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.github.com"}})
		if err != nil {
			t.Fatalf("resolve https proxy failed: %v", err)
		}
		if resolved == nil || resolved.String() != "http://127.0.0.1:7890" {
			t.Fatalf("unexpected resolved proxy: %v", resolved)
		}
	})
}

func TestBuildProxyFunc(t *testing.T) {
	t.Run("explicit https proxy and environment fallback", func(t *testing.T) {
		t.Setenv("HTTP_PROXY", "http://127.0.0.1:18080")
		t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18443")

		proxyFunc, err := buildProxyFunc("http://127.0.0.1:28080")
		if err != nil {
			t.Fatalf("build proxy func failed: %v", err)
		}

		httpProxy, err := proxyFunc(&http.Request{URL: &url.URL{Scheme: "http", Host: "api.github.com"}})
		if err != nil {
			t.Fatalf("resolve http proxy failed: %v", err)
		}
		if httpProxy == nil || httpProxy.String() != "http://127.0.0.1:18080" {
			t.Fatalf("unexpected http proxy: %v", httpProxy)
		}

		httpsProxy, err := proxyFunc(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.github.com"}})
		if err != nil {
			t.Fatalf("resolve https proxy failed: %v", err)
		}
		if httpsProxy == nil || httpsProxy.String() != "http://127.0.0.1:28080" {
			t.Fatalf("unexpected https proxy: %v", httpsProxy)
		}
	})
}
