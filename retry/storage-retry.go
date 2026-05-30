package retry

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

// reqproof:implements SW-REQ-031
func GetTemporalStorageExponentialBackoff() *backoff.ExponentialBackOff {
	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.Multiplier = 2
	exponentialBackoff.MaxInterval = 10 * time.Second
	exponentialBackoff.MaxElapsedTime = 0

	return exponentialBackoff
}
