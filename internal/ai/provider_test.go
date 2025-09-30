package ai

import (
	"testing"
)

func TestProviderError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProviderError
		expected string
	}{
		{
			name: "error with wrapped error",
			err: &ProviderError{
				Provider: "test",
				Message:  "failed",
				Err:      &ProviderError{Provider: "inner", Message: "inner error"},
			},
			expected: "test: failed: inner: inner error",
		},
		{
			name: "error without wrapped error",
			err: &ProviderError{
				Provider: "test",
				Message:  "failed",
			},
			expected: "test: failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("ProviderError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProviderErrorUnwrap(t *testing.T) {
	innerErr := &ProviderError{Provider: "inner", Message: "inner error"}
	outerErr := &ProviderError{
		Provider: "outer",
		Message:  "outer error",
		Err:      innerErr,
	}

	if unwrapped := outerErr.Unwrap(); unwrapped != innerErr {
		t.Errorf("ProviderError.Unwrap() = %v, want %v", unwrapped, innerErr)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limit error",
			err:      &ProviderError{Provider: "test", Message: "rate limit exceeded"},
			expected: true,
		},
		{
			name:     "timeout error",
			err:      &ProviderError{Provider: "test", Message: "timeout occurred"},
			expected: true,
		},
		{
			name:     "connection error",
			err:      &ProviderError{Provider: "test", Message: "connection failed"},
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      &ProviderError{Provider: "test", Message: "invalid API key"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.expected {
				t.Errorf("isRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}