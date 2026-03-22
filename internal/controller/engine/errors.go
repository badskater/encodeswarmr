package engine

import "strings"

// ErrorCategory classifies a task failure to determine retry eligibility.
type ErrorCategory string

const (
	// ErrorCategoryTransient represents errors that are likely temporary and
	// safe to retry, such as network failures, disk full, or generic exit codes.
	ErrorCategoryTransient ErrorCategory = "transient"

	// ErrorCategoryPermanent represents errors that indicate a fundamental
	// problem unlikely to be resolved by retrying, such as codec not found or
	// invalid input data.
	ErrorCategoryPermanent ErrorCategory = "permanent"

	// ErrorCategoryUnknown represents errors that could not be classified. These
	// are retried with a reduced limit (max_retries/2).
	ErrorCategoryUnknown ErrorCategory = "unknown"
)

// transientExitCodes are exit codes that indicate a transient failure.
var transientExitCodes = map[int]bool{
	1: true, // generic error — may be temporary
}

// permanentExitCodes are exit codes that indicate a permanent failure.
// Exit code 2+ (usage/argument errors) are not retried.
var permanentExitCodes = map[int]bool{
	2: true,
}

// transientPhrases are substrings in the error message that indicate a
// transient failure.
var transientPhrases = []string{
	"connection refused",
	"disk full",
	"no space left",
	"signal: killed",
	"signal killed",
	"i/o timeout",
	"network unreachable",
	"temporary failure",
	"resource temporarily unavailable",
	"output validation failed",
}

// permanentPhrases are substrings in the error message that indicate a
// permanent failure.
var permanentPhrases = []string{
	"codec not found",
	"invalid data",
	"no such file",
	"invalid option",
	"unrecognized option",
	"not supported",
	"permission denied",
	"access denied",
	"invalid argument",
	"moov atom not found",
}

// ClassifyError determines the ErrorCategory for a failed task given its exit
// code and error message.
func ClassifyError(exitCode int, errorMsg string) ErrorCategory {
	lower := strings.ToLower(errorMsg)

	// Exit code 0 with a failure report is transient (e.g. output validation
	// failed after the process reported success).
	if exitCode == 0 {
		return ErrorCategoryTransient
	}

	// Check exit code mappings first.
	if transientExitCodes[exitCode] {
		// Still check message for permanent patterns — message takes priority.
		for _, phrase := range permanentPhrases {
			if strings.Contains(lower, phrase) {
				return ErrorCategoryPermanent
			}
		}
		return ErrorCategoryTransient
	}

	if permanentExitCodes[exitCode] || exitCode > 2 {
		return ErrorCategoryPermanent
	}

	// Check message patterns.
	for _, phrase := range permanentPhrases {
		if strings.Contains(lower, phrase) {
			return ErrorCategoryPermanent
		}
	}
	for _, phrase := range transientPhrases {
		if strings.Contains(lower, phrase) {
			return ErrorCategoryTransient
		}
	}

	return ErrorCategoryUnknown
}

// MaxRetriesForCategory returns the maximum number of retries permitted for a
// given error category, given the job-level maxRetries setting.
//
//   - transient: full maxRetries
//   - unknown:   maxRetries / 2 (rounded down, minimum 1 when maxRetries > 0)
//   - permanent: 0 (never retry)
func MaxRetriesForCategory(category ErrorCategory, maxRetries int) int {
	switch category {
	case ErrorCategoryPermanent:
		return 0
	case ErrorCategoryUnknown:
		if maxRetries <= 0 {
			return 0
		}
		half := maxRetries / 2
		if half < 1 {
			return 1
		}
		return half
	default: // transient
		return maxRetries
	}
}
