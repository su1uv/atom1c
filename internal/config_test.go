package internal

import (
	"testing"
	"time"
)

func TestLoadRefreshInterval(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "unset defaults to fifteen minutes", want: 15 * time.Minute},
		{name: "positive duration", value: "45s", want: 45 * time.Second},
		{name: "zero disables", value: "0", want: 0},
		{name: "zero duration disables", value: "0s", want: 0},
		{name: "negative rejected", value: "-1m", wantErr: true},
		{name: "malformed rejected", value: "later", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LoadRefreshInterval(func(key string) string {
				if key != "ATOM1C_REFRESH_INTERVAL" {
					t.Fatalf("lookup key = %q, want ATOM1C_REFRESH_INTERVAL", key)
				}
				return test.value
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("LoadRefreshInterval() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("LoadRefreshInterval() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLoadRefreshIntervalRejectsNilLookup(t *testing.T) {
	if _, err := LoadRefreshInterval(nil); err == nil {
		t.Fatal("LoadRefreshInterval(nil) succeeded, want error")
	}
}
