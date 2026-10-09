package internal

import (
	"testing"
	"time"
)

func TestLoadRefreshInterval(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		set     bool
		want    time.Duration
		wantErr bool
	}{
		{name: "unset defaults to fifteen minutes", want: 15 * time.Minute},
		{name: "positive duration", value: "45s", set: true, want: 45 * time.Second},
		{name: "zero disables", value: "0", set: true, want: 0},
		{name: "zero duration disables", value: "0s", set: true, want: 0},
		{name: "explicit empty rejected", set: true, wantErr: true},
		{name: "negative rejected", value: "-1m", set: true, wantErr: true},
		{name: "malformed rejected", value: "later", set: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LoadRefreshInterval(func(key string) (string, bool) {
				if key != "ATOM1C_REFRESH_INTERVAL" {
					t.Fatalf("lookup key = %q, want ATOM1C_REFRESH_INTERVAL", key)
				}
				return test.value, test.set
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
