package reconciler

import "time"

const (
	// DefaultLoopTimeout is the default timeout for a reconcile loop.
	DefaultLoopTimeout = 90 * time.Minute
	// DefaultMappingTimeout is the default timeout for a controller request mapping func.
	DefaultMappingTimeout = 60 * time.Second
)

// DefaultedLoopTimeout will default the timeout if it is zero-valued.
func DefaultedLoopTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultLoopTimeout
	}

	return timeout
}
