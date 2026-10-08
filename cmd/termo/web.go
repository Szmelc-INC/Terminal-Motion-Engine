package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/app"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/fetch"
)

func keysFrom(p *app.Prefs) fetch.Keys {
	k := fetch.Keys{YouTube: p.YouTubeKey, Giphy: p.GiphyKey}
	if k.YouTube == "" {
		if k.YouTube = os.Getenv("TERMO_YOUTUBE_KEY"); k.YouTube == "" {
			k.YouTube = os.Getenv("YOUTUBE_API_KEY")
		}
	}
	if k.Giphy == "" {
		k.Giphy = os.Getenv("GIPHY_API_KEY")
	}
	return k
}

func findProvider(name string) (*fetch.Provider, error) {
	if name == "" {
		return fetch.Providers[0], nil
	}
	var names []string
	for _, p := range fetch.Providers {
		if strings.EqualFold(p.Name, name) || strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(name)) {
			return p, nil
		}
		names = append(names, strings.ToLower(p.Name))
	}
	return nil, fmt.Errorf("--site: %q is not one of %s", name, strings.Join(names, ", "))
}

func interruptible() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func (c *cli) query() fetch.Query {
	q := fetch.Query{Text: strings.Join(c.args, " "), Kind: "any", Duration: "any", Sort: "relevance"}
	if v, ok := c.get("kind"); ok {
		q.Kind = v
	}
	if v, ok := c.get("length"); ok {
		q.Duration = v
	}
	if v, ok := c.get("sort-by"); ok {
		q.Sort = v
	}
	if n, _ := c.float("count", 0); n > 0 {
		q.Limit = int(n)
	}
	return q
}

// cmdSearch prints search results, one per line.
func cmdSearch(c *cli, prefs *app.Prefs) error {
	if len(c.args) == 0 {
		return errors.New("usage: termo search [--site NAME] [--kind K] [--length L] [--sort-by S] [--count N] WORDS…")
	}
	site, _ := c.get("site")
	p, err := findProvider(site)
	if err != nil {
		return err
	}
	ctx, cancel := interruptible()
	defer cancel()
	items, err := p.Search(ctx, c.query(), keysFrom(prefs))
	if err != nil {
		return err
	}
	for i, it := range items {
		meta := strings.TrimSpace(fetch.Clock(it.Duration) + "  " + fetch.Count(it.Views))
		if meta == "" {
			meta = it.Kind
		}
		fmt.Printf("%2d. %s\n    %s · %s · %s\n    %s\n", i+1, it.Title, it.Provider, meta, it.Author, it.URL)
	}
	if len(items) == 0 {
		return errors.New("nothing found")
	}
	return nil
}

// pick turns the command line into one item: a link as it is, or the first
// (or --pick N-th) result of a search.
func pick(ctx context.Context, c *cli, prefs *app.Prefs) (fetch.Item, error) {
	if len(c.args) == 0 {
		return fetch.Item{}, errors.New("give a link or the words to search for")
	}
	site, _ := c.get("site")
	if fetch.IsURL(c.args[0]) {
		site = "URL"
	}
	p, err := findProvider(site)
	if err != nil {
		return fetch.Item{}, err
	}
	items, err := p.Search(ctx, c.query(), keysFrom(prefs))
	if err != nil {
		return fetch.Item{}, err
	}
	n, _ := c.float("pick", 1)
	if int(n) < 1 || int(n) > len(items) {
		return fetch.Item{}, fmt.Errorf("there is no result number %d (%d found)", int(n), len(items))
	}
	return items[int(n)-1], nil
}

// cmdGet downloads a link, or the best match of a search, and prints the
// path of the saved file.
func cmdGet(c *cli, prefs *app.Prefs) error {
	as, _ := c.get("as")
	if as == "" {
		as = "mp4"
	}
	f, ok := fetch.FindFormat(as)
	if !ok {
		var ids []string
		for _, x := range fetch.Formats {
			ids = append(ids, x.ID)
		}
		return fmt.Errorf("--as: %q is not one of %s", as, strings.Join(ids, ", "))
	}
	dir := prefs.Downloads()
	if v, ok := c.get("dir"); ok {
		dir = v
	}
	ctx, cancel := interruptible()
	defer cancel()
	it, err := pick(ctx, c, prefs)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s\n", it.Title)
	last := time.Now()
	path, err := fetch.Download(ctx, it, f, dir, func(p fetch.Progress) {
		if time.Since(last) < 250*time.Millisecond {
			return
		}
		last = time.Now()
		pct := ""
		if p.Percent >= 0 {
			pct = fmt.Sprintf("%5.1f%%", p.Percent)
		}
		fmt.Fprintf(os.Stderr, "\r\x1b[K%s %s %s %s", p.Stage, pct, p.Speed, p.ETA)
	})
	fmt.Fprint(os.Stderr, "\r\x1b[K")
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// resolveArgs replaces links on the command line with their stream URLs,
// so "termo https://youtu.be/…" just plays.
func resolveArgs(args []string) ([]string, []app.Stream, error) {
	var streams []app.Stream
	out := append([]string(nil), args...)
	for i, a := range args {
		if !fetch.IsURL(a) {
			continue
		}
		fmt.Fprintf(os.Stderr, "opening %s …\n", a)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		items, err := fetch.Providers[len(fetch.Providers)-1].Search(ctx, fetch.Query{Text: a}, fetch.Keys{})
		if err != nil || len(items) == 0 {
			cancel()
			if err == nil {
				err = errors.New("nothing playable there")
			}
			return nil, nil, fmt.Errorf("%s: %v", a, err)
		}
		st, err := fetch.Resolve(ctx, items[0], 480)
		cancel()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", a, err)
		}
		out[i] = st.Video
		streams = append(streams, app.Stream{Path: st.Video, Audio: st.Audio, Name: items[0].Title})
	}
	return out, streams, nil
}
