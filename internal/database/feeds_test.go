package database

import (
	"context"
	"fmt"
	"testing"
)

func TestGetFeedsPageIncludesEveryPageAndStableOrdering(t *testing.T) {
	db, queries, _ := openTimestampTestDB(t)
	defer db.Close()
	ctx := context.Background()

	for i := 0; i < 23; i++ {
		if _, err := queries.CreateFeed(ctx, CreateFeedParams{
			Name: fmt.Sprintf("Feed %02d", i),
			Url:  fmt.Sprintf("https://example.test/%02d", i),
		}); err != nil {
			t.Fatalf("create feed %d: %v", i, err)
		}
	}

	first, err := queries.GetFeedsPage(ctx, GetFeedsPageParams{Search: "", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("get first page: %v", err)
	}
	second, err := queries.GetFeedsPage(ctx, GetFeedsPageParams{Search: "", Limit: 10, Offset: 10})
	if err != nil {
		t.Fatalf("get second page: %v", err)
	}
	third, err := queries.GetFeedsPage(ctx, GetFeedsPageParams{Search: "", Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("get third page: %v", err)
	}

	if len(first)+len(second)+len(third) != 23 {
		t.Fatalf("paged feed count = %d, want all 23 feeds", len(first)+len(second)+len(third))
	}
	if first[0].Name != "Feed 00" || second[0].Name != "Feed 10" || third[0].Name != "Feed 20" {
		t.Fatalf("page starts = %q, %q, %q; want Feed 00, Feed 10, Feed 20", first[0].Name, second[0].Name, third[0].Name)
	}
	if first[len(first)-1].ID >= second[0].ID {
		t.Fatalf("feeds with tied timestamps are not ordered by ID: %d then %d", first[len(first)-1].ID, second[0].ID)
	}

	count, err := queries.CountFeeds(ctx, "")
	if err != nil {
		t.Fatalf("count feeds: %v", err)
	}
	if count != 23 {
		t.Fatalf("count = %d, want 23", count)
	}
}

func TestGetFeedsPageSearchIsGlobalCaseInsensitiveAndLiteral(t *testing.T) {
	db, queries, _ := openTimestampTestDB(t)
	defer db.Close()
	ctx := context.Background()

	fixtures := []struct {
		name string
		url  string
	}{
		{name: "Alpha news", url: "https://example.test/alpha"},
		{name: "Middle", url: "https://example.test/middle"},
		{name: "A%_B source", url: "https://example.test/literal"},
		{name: "alpha archive", url: "https://example.test/archive"},
		{name: "ÄLFA source", url: "https://example.test/unicode"},
	}
	for _, fixture := range fixtures {
		if _, err := queries.CreateFeed(ctx, CreateFeedParams{Name: fixture.name, Url: fixture.url}); err != nil {
			t.Fatalf("create feed %q: %v", fixture.name, err)
		}
	}

	for _, tc := range []struct {
		name      string
		query     string
		limit     int64
		offset    int64
		wantNames []string
		wantCount int64
	}{
		{name: "case insensitive beyond first page", query: "ALPHA", limit: 1, offset: 1, wantNames: []string{"alpha archive"}, wantCount: 2},
		{name: "unicode case insensitive", query: "älfa", limit: 10, offset: 0, wantNames: []string{"ÄLFA source"}, wantCount: 1},
		{name: "literal SQL wildcard characters", query: "%_", limit: 10, offset: 0, wantNames: []string{"A%_B source"}, wantCount: 1},
		{name: "empty search matches all", query: "", limit: 10, offset: 0, wantNames: []string{"Alpha news", "Middle", "A%_B source", "alpha archive", "ÄLFA source"}, wantCount: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := queries.GetFeedsPage(ctx, GetFeedsPageParams{
				Search: tc.query,
				Limit:  tc.limit,
				Offset: tc.offset,
			})
			if err != nil {
				t.Fatalf("get page: %v", err)
			}
			if len(got) != len(tc.wantNames) {
				t.Fatalf("page has %d feeds, want %d: %#v", len(got), len(tc.wantNames), got)
			}
			for i, want := range tc.wantNames {
				if got[i].Name != want {
					t.Errorf("feed %d = %q, want %q", i, got[i].Name, want)
				}
			}
			count, err := queries.CountFeeds(ctx, tc.query)
			if err != nil {
				t.Fatalf("count search results: %v", err)
			}
			if count != tc.wantCount {
				t.Errorf("matching count = %d, want %d", count, tc.wantCount)
			}
		})
	}
}

func TestGetFeedPositionUsesSearchAndStableOrder(t *testing.T) {
	db, queries, _ := openTimestampTestDB(t)
	defer db.Close()
	ctx := context.Background()

	first, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "News First", Url: "https://example.test/first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "Other", Url: "https://example.test/other"}); err != nil {
		t.Fatal(err)
	}
	last, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "News Last", Url: "https://example.test/last"})
	if err != nil {
		t.Fatal(err)
	}

	position, err := queries.GetFeedPosition(ctx, GetFeedPositionParams{Search: "news", ID: last.ID})
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if position != 1 {
		t.Fatalf("position = %d, want 1 among search results", position)
	}
	position, err = queries.GetFeedPosition(ctx, GetFeedPositionParams{Search: "", ID: first.ID})
	if err != nil {
		t.Fatalf("get first position: %v", err)
	}
	if position != 0 {
		t.Fatalf("first feed position = %d, want 0", position)
	}
}
