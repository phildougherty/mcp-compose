package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	MaxRequestBodySize = 10 * 1024 * 1024
	DefaultBodyLimit   = MaxRequestBodySize
)

type ValidationConfig struct {
	MaxBodySize           int64
	RequireContentType    bool
	AllowedContentTypes   []string
	StripHTML             bool
	ValidateJSON          bool
	RequireJSONRPC        bool
	CustomValidators      map[string]RequestValidator
}

type ValidationError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

type RequestValidator func(*http.Request, []byte) []ValidationError

func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxBodySize:        DefaultBodyLimit,
		RequireContentType: true,
		AllowedContentTypes: []string{
			"application/json",
			"application/json; charset=utf-8",
			"application/json;charset=utf-8",
		},
		StripHTML:        true,
		ValidateJSON:     true,
		RequireJSONRPC:   false,
		CustomValidators: make(map[string]RequestValidator),
	}
}

type RequestValidationMiddleware struct {
	config *ValidationConfig
	logger *logging.Logger
}

func NewRequestValidationMiddleware(cfg *ValidationConfig, logger *logging.Logger) *RequestValidationMiddleware {
	if cfg == nil {
		cfg = DefaultValidationConfig()
	}

	if logger == nil {
		logger = logging.NewLogger("info")
	}

	return &RequestValidationMiddleware{
		config: cfg,
		logger: logger,
	}
}

func (v *RequestValidationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)

			return
		}

		if v.config.RequireContentType && r.Method != http.MethodDelete {
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				v.sendValidationError(w, []ValidationError{{
					Field:   "Content-Type",
					Message: "Content-Type header is required",
					Code:    "missing_content_type",
				}})

				return
			}

			if !v.isAllowedContentType(contentType) {
				v.sendValidationError(w, []ValidationError{{
					Field:   "Content-Type",
					Message: fmt.Sprintf("Content-Type must be one of: %s", strings.Join(v.config.AllowedContentTypes, ", ")),
					Code:    "invalid_content_type",
				}})

				return
			}
		}

		if r.ContentLength > v.config.MaxBodySize {
			v.sendValidationError(w, []ValidationError{{
				Field:   "body",
				Message: fmt.Sprintf("Request body too large (max: %d bytes)", v.config.MaxBodySize),
				Code:    "body_too_large",
			}})

			return
		}

		if r.Body != nil && r.Body != http.NoBody {
			body, err := io.ReadAll(io.LimitReader(r.Body, v.config.MaxBodySize+1))
			if err != nil {
				v.sendValidationError(w, []ValidationError{{
					Message: "Failed to read request body",
					Code:    "body_read_error",
				}})

				return
			}

			if err := r.Body.Close(); err != nil {
				v.logger.Warning("Failed to close request body: %v", err)
			}

			if int64(len(body)) > v.config.MaxBodySize {
				v.sendValidationError(w, []ValidationError{{
					Field:   "body",
					Message: fmt.Sprintf("Request body too large (max: %d bytes)", v.config.MaxBodySize),
					Code:    "body_too_large",
				}})

				return
			}

			if v.config.ValidateJSON && len(body) > 0 {
				validationErrors := v.validateJSONBody(body)
				if len(validationErrors) > 0 {
					v.sendValidationError(w, validationErrors)

					return
				}
			}

			if v.config.StripHTML && len(body) > 0 {
				body = stripHTMLTags(body)
			}

			if v.config.RequireJSONRPC && len(body) > 0 {
				validationErrors := v.validateJSONRPCRequest(body)
				if len(validationErrors) > 0 {
					v.sendValidationError(w, validationErrors)

					return
				}
			}

			if customValidator, exists := v.config.CustomValidators[r.URL.Path]; exists {
				validationErrors := customValidator(r, body)
				if len(validationErrors) > 0 {
					v.sendValidationError(w, validationErrors)

					return
				}
			}

			r.Body = io.NopCloser(bytes.NewBuffer(body))
			r.ContentLength = int64(len(body))
		}

		next.ServeHTTP(w, r)
	})
}

func (v *RequestValidationMiddleware) isAllowedContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))

	for _, allowed := range v.config.AllowedContentTypes {
		if strings.HasPrefix(contentType, strings.ToLower(allowed)) {
			return true
		}
	}

	return false
}

func (v *RequestValidationMiddleware) validateJSONBody(body []byte) []ValidationError {
	var js json.RawMessage
	if err := json.Unmarshal(body, &js); err != nil {
		return []ValidationError{{
			Field:   "body",
			Message: fmt.Sprintf("Invalid JSON: %s", sanitizeErrorMessage(err.Error())),
			Code:    "invalid_json",
		}}
	}

	return nil
}

func (v *RequestValidationMiddleware) validateJSONRPCRequest(body []byte) []ValidationError {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return []ValidationError{{
			Field:   "body",
			Message: "Invalid JSON-RPC request format",
			Code:    "invalid_jsonrpc",
		}}
	}

	errors := []ValidationError{}

	if jsonrpc, ok := req["jsonrpc"].(string); !ok || jsonrpc != "2.0" {
		errors = append(errors, ValidationError{
			Field:   "jsonrpc",
			Message: "JSON-RPC version must be '2.0'",
			Code:    "invalid_jsonrpc_version",
		})
	}

	if _, ok := req["method"].(string); !ok {
		errors = append(errors, ValidationError{
			Field:   "method",
			Message: "JSON-RPC method is required and must be a string",
			Code:    "missing_method",
		})
	}

	return errors
}

func (v *RequestValidationMiddleware) sendValidationError(w http.ResponseWriter, errors []ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    400,
			"message": "Request validation failed",
			"details": ValidationErrors{Errors: errors},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		v.logger.Error("Failed to encode validation error response: %v", err)
	}

	v.logger.Debug("Request validation failed: %d errors", len(errors))
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(data []byte) []byte {
	return htmlTagRegex.ReplaceAll(data, []byte(""))
}

func sanitizeErrorMessage(msg string) string {
	msg = strings.ReplaceAll(msg, "<", "&lt;")
	msg = strings.ReplaceAll(msg, ">", "&gt;")

	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}

	return msg
}

func ValidateServerName(name string) error {
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("server name too long (max: 255 characters)")
	}

	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("server name contains invalid characters (allowed: a-z, A-Z, 0-9, _, -)")
	}

	return nil
}

func ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if len(sessionID) > 128 {
		return fmt.Errorf("session ID too long (max: 128 characters)")
	}

	validSessionRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validSessionRegex.MatchString(sessionID) {
		return fmt.Errorf("session ID contains invalid characters")
	}

	return nil
}

func ValidateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("tool name too long (max: 255 characters)")
	}

	return nil
}

func encodeJSON(w http.ResponseWriter, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}