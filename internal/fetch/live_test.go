package fetch

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveProviders talks to the real sites. It only runs when TERMO_LIVE=1
// is set, so the normal test run stays offline:
//
//	TERMO_LIVE=1 go test ./internal/fetch -run Live -v
func TestLiveProviders(t *testing.T) {
	if os.Getenv("TERMO_LIVE") == "" {
		t.Skip("set TERMO_LIVE=1 to search the real sites")
	}
	keys := Keys{YouTube: os.Getenv("TERMO_YOUTUBE_KEY"), Giphy: os.Getenv("GIPHY_API_KEY")}
	for _, p := range Providers {
		if p.Name == "URL" {
			continue
		}
		t.Run(p.Name, func(t *testing.T) {
			if m := p.Missing(); m != "" {
				t.Skipf("%s is not installed", m)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			items, err := p.Search(ctx, Query{Text: "funny cat", Kind: "any", Duration: "any", Sort: "relevance"}, keys)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) == 0 {
				t.Fatal("no results")
			}
			for _, it := range items[:min(3, len(items))] {
				if it.Title == "" || !IsURL(it.URL) {
					t.Errorf("incomplete result: %+v", it)
				}
			}
			t.Logf("%d results, first: %q %s", len(items), items[0].Title, items[0].URL)
		})
	}
}
