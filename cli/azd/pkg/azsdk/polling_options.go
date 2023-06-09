package azsdk

import "time"

// DefaultPollFrequency is the suggested default polling frequency.
var DefaultPollFrequency = 30 * time.Second

// Always -1, which means unset. In tests, this is used to control
// the polling frequency for all polling clients calling PollFrequency.
var FixedPollFrequency = -1 * time.Second

// PollFrequency returns the polling frequency to use for long-running operations.
func PollFrequency(freq time.Duration) time.Duration {
	if FixedPollFrequency > 0 {
		return FixedPollFrequency
	}

	return freq
}
