package internal

import (
	"fmt"
	"time"
)

// LoadRefreshInterval reads the server's automatic feed-refresh interval. An
// unset value defaults to 15 minutes; zero disables scheduled refresh.
func LoadRefreshInterval(lookup func(string) (string, bool)) (time.Duration, error) {
	if lookup == nil {
		return 0, fmt.Errorf("refresh interval environment lookup is nil")
	}
	value, set := lookup("ATOM1C_REFRESH_INTERVAL")
	if !set {
		return 15 * time.Minute, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid ATOM1C_REFRESH_INTERVAL %q: %w", value, err)
	}
	if interval < 0 {
		return 0, fmt.Errorf("invalid ATOM1C_REFRESH_INTERVAL %q: duration must not be negative", value)
	}
	return interval, nil
}
