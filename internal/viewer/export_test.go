package viewer

import "time"

// SetFollowIdleDeadline overrides followIdleDeadline for tests. The returned
// function restores the prior value.
func SetFollowIdleDeadline(d time.Duration) func() {
	prev := followIdleDeadline
	followIdleDeadline = d
	return func() { followIdleDeadline = prev }
}
