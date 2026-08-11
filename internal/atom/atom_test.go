package atom

import (
	"context"
	"log"
	"testing"
)

func TestFetchFeed(t *testing.T) {
	tests := map[string]struct {
		input string
		want  bool
	}{
		"simple": {
			input: "https://hnrss.org/newest.atom",
			want:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := fetchFeed(context.Background(), tc.input)
			if err != nil {
				log.Fatal(err)
			}
		})
	}
}
