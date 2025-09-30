package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

func TestRequestValidation_ContentType(t *testing.T) {
	cfg := DefaultValidationConfig()
	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "valid content type",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "valid content type with charset",
			contentType: "application/json; charset=utf-8",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "invalid content type",
			contentType: "text/html",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing content type",
			contentType: "",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			body := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestValidation_BodySize(t *testing.T) {
	cfg := &ValidationConfig{
		MaxBodySize:        100,
		RequireContentType: false,
		ValidateJSON:       false,
	}

	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name       string
		bodySize   int
		wantStatus int
	}{
		{
			name:       "body within limit",
			bodySize:   50,
			wantStatus: http.StatusOK,
		},
		{
			name:       "body at limit",
			bodySize:   100,
			wantStatus: http.StatusOK,
		},
		{
			name:       "body exceeds limit",
			bodySize:   101,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			body := bytes.Repeat([]byte("a"), tt.bodySize)
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestValidation_JSON(t *testing.T) {
	cfg := DefaultValidationConfig()
	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid JSON",
			body:       `{"key":"value"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid JSON array",
			body:       `[1,2,3]`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid JSON",
			body:       `{invalid json}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON",
			body:       `{"key":}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestValidation_JSONRPC(t *testing.T) {
	cfg := DefaultValidationConfig()
	cfg.RequireJSONRPC = true

	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid JSON-RPC",
			body: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "test",
				"id":      1,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing jsonrpc version",
			body: map[string]interface{}{
				"method": "test",
				"id":     1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid jsonrpc version",
			body: map[string]interface{}{
				"jsonrpc": "1.0",
				"method":  "test",
				"id":      1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing method",
			body: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestValidation_HTMLStripping(t *testing.T) {
	cfg := DefaultValidationConfig()
	cfg.RequireContentType = false
	cfg.ValidateJSON = false

	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple HTML tags",
			input:    `<script>alert('xss')</script>`,
			expected: `alert('xss')`,
		},
		{
			name:     "nested HTML tags",
			input:    `<div><p>test</p></div>`,
			expected: `test`,
		},
		{
			name:     "no HTML tags",
			input:    `plain text`,
			expected: `plain text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := make([]byte, len(tt.input))
				n, _ := r.Body.Read(body)
				receivedBody = string(body[:n])
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.input))

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if receivedBody != tt.expected {
				t.Errorf("Expected body %q, got %q", tt.expected, receivedBody)
			}
		})
	}
}

func TestRequestValidation_CustomValidator(t *testing.T) {
	cfg := DefaultValidationConfig()
	cfg.RequireContentType = false
	cfg.ValidateJSON = false

	cfg.CustomValidators["/custom"] = func(r *http.Request, body []byte) []ValidationError {
		if string(body) != "valid" {
			return []ValidationError{{
				Message: "Custom validation failed",
				Code:    "custom_error",
			}}
		}

		return nil
	}

	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid custom endpoint",
			path:       "/custom",
			body:       "valid",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid custom endpoint",
			path:       "/custom",
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-custom endpoint",
			path:       "/other",
			body:       "anything",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := validator.Middleware(handler)

			req := httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body))

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestValidation_GETRequestsSkipped(t *testing.T) {
	cfg := DefaultValidationConfig()
	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := validator.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for GET request, got %d", w.Code)
	}
}

func TestValidateServerName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid name",
			input:     "my-server-1",
			wantError: false,
		},
		{
			name:      "valid name with underscore",
			input:     "my_server",
			wantError: false,
		},
		{
			name:      "empty name",
			input:     "",
			wantError: true,
		},
		{
			name:      "name too long",
			input:     strings.Repeat("a", 256),
			wantError: true,
		},
		{
			name:      "invalid characters",
			input:     "my server!",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerName(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateServerName() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid session ID",
			input:     "abc123-def456",
			wantError: false,
		},
		{
			name:      "empty session ID",
			input:     "",
			wantError: true,
		},
		{
			name:      "session ID too long",
			input:     strings.Repeat("a", 129),
			wantError: true,
		},
		{
			name:      "invalid characters",
			input:     "abc 123",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSessionID() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func BenchmarkRequestValidation_JSON(b *testing.B) {
	cfg := DefaultValidationConfig()
	logger := logging.NewLogger("error")
	validator := NewRequestValidationMiddleware(cfg, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := validator.Middleware(handler)

	body := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req.Body = io.NopCloser(bytes.NewReader(body))
		middleware.ServeHTTP(w, req)
	}
}