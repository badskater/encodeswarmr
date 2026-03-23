package engine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestClassifyError
// ---------------------------------------------------------------------------

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		errorMsg string
		want     ErrorCategory
	}{
		{
			name:     "exit 1 + connection refused → transient",
			exitCode: 1,
			errorMsg: "connection refused",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 2 + codec not found → permanent",
			exitCode: 2,
			errorMsg: "codec not found",
			want:     ErrorCategoryPermanent,
		},
		{
			name:     "exit 1 + no space left → transient",
			exitCode: 1,
			errorMsg: "no space left on device",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 0 + signal: killed → transient",
			exitCode: 0,
			errorMsg: "signal: killed",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 3 + empty → permanent (exit > 2)",
			exitCode: 3,
			errorMsg: "",
			want:     ErrorCategoryPermanent,
		},
		{
			// Exit code 1 is in transientExitCodes, so it returns transient
			// when the message doesn't match any permanent phrase.
			name:     "exit 1 + unknown error → transient (exit code 1 is transient)",
			exitCode: 1,
			errorMsg: "unknown error occurred",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 0 + empty → transient",
			exitCode: 0,
			errorMsg: "",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 1 + invalid data → permanent (message overrides exit code)",
			exitCode: 1,
			errorMsg: "invalid data found",
			want:     ErrorCategoryPermanent,
		},
		{
			name:     "exit 1 + permission denied → permanent (message overrides exit code)",
			exitCode: 1,
			errorMsg: "permission denied",
			want:     ErrorCategoryPermanent,
		},
		{
			name:     "exit 2 + no message → permanent",
			exitCode: 2,
			errorMsg: "",
			want:     ErrorCategoryPermanent,
		},
		{
			name:     "exit 1 + i/o timeout → transient",
			exitCode: 1,
			errorMsg: "i/o timeout reading file",
			want:     ErrorCategoryTransient,
		},
		{
			name:     "exit 100 + empty → permanent (exit > 2)",
			exitCode: 100,
			errorMsg: "",
			want:     ErrorCategoryPermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.exitCode, tt.errorMsg)
			if got != tt.want {
				t.Errorf("ClassifyError(%d, %q) = %q, want %q",
					tt.exitCode, tt.errorMsg, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMaxRetriesForCategory
// ---------------------------------------------------------------------------

func TestMaxRetriesForCategory(t *testing.T) {
	tests := []struct {
		name       string
		category   ErrorCategory
		maxRetries int
		want       int
	}{
		{
			name:       "permanent always returns 0",
			category:   ErrorCategoryPermanent,
			maxRetries: 10,
			want:       0,
		},
		{
			name:       "permanent with 0 maxRetries returns 0",
			category:   ErrorCategoryPermanent,
			maxRetries: 0,
			want:       0,
		},
		{
			name:       "transient returns full maxRetries",
			category:   ErrorCategoryTransient,
			maxRetries: 5,
			want:       5,
		},
		{
			name:       "transient with 0 maxRetries returns 0",
			category:   ErrorCategoryTransient,
			maxRetries: 0,
			want:       0,
		},
		{
			name:       "unknown returns maxRetries/2",
			category:   ErrorCategoryUnknown,
			maxRetries: 8,
			want:       4,
		},
		{
			name:       "unknown with 1 maxRetries returns 1 (minimum)",
			category:   ErrorCategoryUnknown,
			maxRetries: 1,
			want:       1,
		},
		{
			name:       "unknown with 0 maxRetries returns 0",
			category:   ErrorCategoryUnknown,
			maxRetries: 0,
			want:       0,
		},
		{
			name:       "unknown with odd maxRetries rounds down",
			category:   ErrorCategoryUnknown,
			maxRetries: 7,
			want:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxRetriesForCategory(tt.category, tt.maxRetries)
			if got != tt.want {
				t.Errorf("MaxRetriesForCategory(%q, %d) = %d, want %d",
					tt.category, tt.maxRetries, got, tt.want)
			}
		})
	}
}
