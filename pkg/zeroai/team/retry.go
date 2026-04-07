package team

import (
	"math"
	"math/rand"
	"time"
)

// RetryOptions configures retry behavior
type RetryOptions struct {
	MaxRetries         int                                                   // Maximum number of retry attempts
	BaseDelay          time.Duration                                         // Base delay between retries
	MaxDelay           time.Duration                                         // Maximum delay cap (default: 60s)
	ExponentialBackoff bool                                                  // Use exponential backoff (default: true)
	Jitter             bool                                                  // Add random jitter (default: true)
	OnRetry            func(attempt int, err error, nextDelay time.Duration) // Optional callback
}

// DefaultRetryOptions returns sensible defaults for retry configuration
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxRetries:         3,
		BaseDelay:          1 * time.Second,
		MaxDelay:           60 * time.Second,
		ExponentialBackoff: true,
		Jitter:             true,
	}
}

// CalculateBackoffDelay computes the delay for a given retry attempt.
// attempt is 1-based (first retry = attempt 1).
// baseDelayMs and maxDelayMs are in milliseconds.
// useJitter adds random jitter (0-25% of delay) to prevent thundering herd.
// Returns delay in milliseconds.
func CalculateBackoffDelay(attempt int, baseDelayMs int, maxDelayMs int, useJitter bool) int {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	delay := baseDelayMs * int(math.Pow(2, float64(attempt-1)))

	// Cap at maximum delay
	if delay > maxDelayMs {
		delay = maxDelayMs
	}

	// Add jitter (0-25% of delay) to prevent thundering herd
	if useJitter {
		jitter := int(float64(delay) * 0.25 * rand.Float64())
		delay += jitter
	}

	return delay
}

// WithRetry executes a function with retry logic and exponential backoff.
// Returns the result on success, or the last error after all retries exhausted.
func WithRetry[T any](fn func() (T, error), opts RetryOptions) (T, error) {
	var lastErr error
	maxDelayMs := int(opts.MaxDelay.Milliseconds())
	if maxDelayMs == 0 {
		maxDelayMs = 60000 // Default 60s
	}

	for attempt := 1; attempt <= opts.MaxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if attempt < opts.MaxRetries {
			var delayMs int
			if opts.ExponentialBackoff {
				delayMs = CalculateBackoffDelay(attempt, int(opts.BaseDelay.Milliseconds()), maxDelayMs, opts.Jitter)
			} else {
				delayMs = int(opts.BaseDelay.Milliseconds())
			}

			if opts.OnRetry != nil {
				opts.OnRetry(attempt, err, time.Duration(delayMs)*time.Millisecond)
			}

			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	var zero T
	return zero, lastErr
}

// Sleep is a helper for delayed operations (useful for testing)
var Sleep = func(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
