package team

import (
	"regexp"
)

// retryableErrorPatterns matches errors that are transient and retryable
// (rate limits, network issues, timeouts)
var retryableErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)rate_limit`),
	regexp.MustCompile(`(?i)hit your limit`),
	regexp.MustCompile(`(?i)quota`),
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`429`),
	regexp.MustCompile(`(?i)timeout`),
	regexp.MustCompile(`(?i)network`),
	regexp.MustCompile(`(?i)connection`),
	regexp.MustCompile(`ECONNRESET`),
	regexp.MustCompile(`ETIMEDOUT`),
	regexp.MustCompile(`ENOTFOUND`),
	regexp.MustCompile(`(?i)overloaded`),
}

// fatalErrorPatterns matches errors that indicate configuration or auth problems
// that will NOT be resolved by retrying
var fatalErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)not authenticated`),
	regexp.MustCompile(`(?i)no authentication`),
	regexp.MustCompile(`(?i)authentication failed`),
	regexp.MustCompile(`(?i)invalid.*token`),
	regexp.MustCompile(`(?i)invalid.*api.?key`),
	regexp.MustCompile(`(?i)unauthorized`),
	regexp.MustCompile(`\b401\b`),
	regexp.MustCompile(`\b403\b`),
	regexp.MustCompile(`(?i)command not found`),
	regexp.MustCompile(`(?i)not installed`),
	regexp.MustCompile(`(?i)is not recognized`),
}

// IsRetryableError returns true if the error string matches any retryable pattern.
// These are transient errors (rate limits, network issues) that may resolve on retry.
func IsRetryableError(err string) bool {
	for _, pattern := range retryableErrorPatterns {
		if pattern.MatchString(err) {
			return true
		}
	}
	return false
}

// IsFatalError returns true if the error string matches any fatal pattern.
// These are configuration or authentication errors that will NOT resolve on retry.
func IsFatalError(err string) bool {
	for _, pattern := range fatalErrorPatterns {
		if pattern.MatchString(err) {
			return true
		}
	}
	return false
}

// ClassifyError returns a classification string: "retryable", "fatal", or "unknown"
func ClassifyError(err string) string {
	if IsFatalError(err) {
		return "fatal"
	}
	if IsRetryableError(err) {
		return "retryable"
	}
	return "unknown"
}
