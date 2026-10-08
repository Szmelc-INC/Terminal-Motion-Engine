package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// --- YouTube -------------------------------------------------------------------

// searchYouTube uses the Data API when a key is configured and falls back
// to yt-dlp's own search when there is none or the key's quota is spent.
func searchYouTube(ctx context.Context, q Query, k Keys) ([]Item, error) {
	keys := strings.FieldsFunc(k.YouTube, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
	var apiErr error
	for _, key := range keys {
		items, err := youtubeAPI(ctx, q, key)
		if err == nil {
			return items, nil
		}
		apiErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	items, err := youtubeDLP(ctx, q)
	if err != nil && apiErr != nil {
		return nil, fmt.Errorf("%v (API: %v)", err, apiErr)
	}
	return items, err
}

type ytSearchDoc struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			PublishedAt  string `json:"publishedAt"`
			Description  string `json:"description"`
			Thumbnails   map[string]struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"snippet"`
	} `json:"items"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type ytVideosDoc struct {
	Items []struct {
		ID             string `json:"id"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Statistics struct {
			ViewCount string `json:"viewCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func parseYouTubeSearch(data []byte) ([]Item, error) {
	var doc ytSearchDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("YouTube sent something that is not JSON")
	}
	if doc.Error != nil {
		return nil, errors.New(html.UnescapeString(stripTags(doc.Error.Message)))
	}
	var out []Item
	for _, it := range doc.Items {
		if it.ID.VideoID == "" {
			continue
		}
		thumb := it.Snippet.Thumbnails["medium"].URL
		if thumb == "" {
			thumb = "https://i.ytimg.com/vi/" + it.ID.VideoID + "/mqdefault.jpg"
		}
		date, _, _ := strings.Cut(it.Snippet.PublishedAt, "T")
		out = append(out, Item{Provider: "YouTube", ID: it.ID.VideoID, Kind: "video",
			Title: html.UnescapeString(it.Snippet.Title), Author: html.UnescapeString(it.Snippet.ChannelTitle),
			URL: "https://www.youtube.com/watch?v=" + it.ID.VideoID, Thumb: thumb, Date: date,
			Desc: html.UnescapeString(it.Snippet.Description)})
	}
	return out, nil
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string { return tagRe.ReplaceAllString(s, "") }

var isoDurRe = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// isoDuration parses an ISO 8601 duration such as PT1H2M3S into seconds.
func isoDuration(s string) float64 {
	m := isoDurRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var total float64
	for i, mul := range []float64{86400, 3600, 60, 1} {
		n, _ := strconv.Atoi(m[i+1])
		total += float64(n) * mul
	}
	return total
}

func youtubeAPI(ctx context.Context, q Query, key string) ([]Item, error) {
	v := url.Values{"part": {"snippet"}, "type": {"video"}, "q": {q.Text}, "key": {key},
		"maxResults": {strconv.Itoa(min(50, q.Offset+limit(q, 20)))}, "safeSearch": {"none"}}
	if q.Duration != "" && q.Duration != "any" {
		v.Set("videoDuration", q.Duration)
	}
	switch q.Sort {
	case "date":
		v.Set("order", "date")
	case "views":
		v.Set("order", "viewCount")
	}
	body, err := get(ctx, "https://www.googleapis.com/youtube/v3/search?"+v.Encode())
	if err != nil && body == nil {
		return nil, err
	}
	items, perr := parseYouTubeSearch(body)
	if perr != nil {
		return nil, perr
	}
	if err != nil {
		return nil, err
	}
	if q.Offset < len(items) {
		items = items[q.Offset:]
	} else {
		items = nil
	}
	// One more cheap call fills in what search results lack.
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	var vd ytVideosDoc
	dv := url.Values{"part": {"contentDetails,statistics"}, "id": {strings.Join(ids, ",")}, "key": {key}}
	if len(ids) > 0 && getJSON(ctx, "https://www.googleapis.com/youtube/v3/videos?"+dv.Encode(), &vd) == nil {
		for _, d := range vd.Items {
			for i := range items {
				if items[i].ID == d.ID {
					items[i].Duration = isoDuration(d.ContentDetails.Duration)
					items[i].Views, _ = strconv.ParseInt(d.Statistics.ViewCount, 10, 64)
				}
			}
		}
	}
	return items, nil
}

type dlpEntry struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	WebpageURL string  `json:"webpage_url"`
	Duration   float64 `json:"duration"`
	ViewCount  int64   `json:"view_count"`
	Channel    string  `json:"channel"`
	Uploader   string  `json:"uploader"`
	UploadDate string  `json:"upload_date"`
	Desc       string  `json:"description"`
	Thumbnail  string  `json:"thumbnail"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

func parseDLPSearch(data []byte, provider string) ([]Item, error) {
	var doc struct {
		Entries []dlpEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("yt-dlp sent something that is not JSON")
	}
	var out []Item
	for _, e := range doc.Entries {
		u := e.WebpageURL
		if u == "" {
			u = e.URL
		}
		if u == "" || e.Title == "" {
			continue
		}
		thumb := e.Thumbnail
		if provider == "YouTube" && e.ID != "" {
			thumb = "https://i.ytimg.com/vi/" + e.ID + "/mqdefault.jpg"
		} else if thumb == "" && len(e.Thumbnails) > 0 {
			thumb = e.Thumbnails[len(e.Thumbnails)-1].URL
		}
		author := e.Channel
		if author == "" {
			author = e.Uploader
		}
		date := e.UploadDate
		if len(date) == 8 {
			date = date[:4] + "-" + date[4:6] + "-" + date[6:]
		}
		out = append(out, Item{Provider: provider, ID: e.ID, Kind: "video", Title: e.Title, Author: author, URL: u,
			Thumb: thumb, Duration: e.Duration, Views: e.ViewCount, Date: date, Desc: e.Desc})
	}
	return out, nil
}

func youtubeDLP(ctx context.Context, q Query) ([]Item, error) {
	n := limit(q, 20)
	prefix := "ytsearch"
	if q.Sort == "date" {
		prefix = "ytsearchdate"
	}
	want := q.Offset + n
	if q.Duration != "" && q.Duration != "any" {
		want += n // some results will be filtered out below
	}
	out, err := run(ctx, "yt-dlp", "--flat-playlist", "-J", "--no-warnings",
		"--playlist-items", fmt.Sprintf("%d-%d", q.Offset+1, want), fmt.Sprintf("%s%d:%s", prefix, want, q.Text))
	if err != nil {
		return nil, err
	}
	items, err := parseDLPSearch(out, "YouTube")
	if err != nil {
		return nil, err
	}
	items = keep(items, Query{Duration: q.Duration})
	if len(items) > n {
		items = items[:n]
	}
	return items, nil
}

// --- Giphy -----------------------------------------------------------------------

func giphyItem(id, title, author string) Item {
	return Item{Provider: "Giphy", ID: id, Kind: "gif", Title: title, Author: author, Direct: true, Ext: ".mp4",
		URL: "https://i.giphy.com/media/" + id + "/giphy.mp4", Thumb: "https://i.giphy.com/media/" + id + "/200_s.gif"}
}

var giphySlugRe = regexp.MustCompile(`giphy\.com/gifs/([A-Za-z0-9-]*?)-?([A-Za-z0-9]{10,24})["?/\\]`)

// parseGiphyHTML pulls results out of a giphy.com search page, which is
// what the search falls back to without an API key.
func parseGiphyHTML(page string) []Item {
	var out []Item
	seen := map[string]bool{}
	for _, m := range giphySlugRe.FindAllStringSubmatch(page, -1) {
		slug, id := m[1], m[2]
		if seen[id] || slug == "" {
			continue
		}
		seen[id] = true
		out = append(out, giphyItem(id, strings.ReplaceAll(slug, "-", " "), ""))
	}
	return out
}

func searchGiphy(ctx context.Context, q Query, k Keys) ([]Item, error) {
	if key := strings.TrimSpace(k.Giphy); key != "" {
		var doc struct {
			Data []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Username string `json:"username"`
				Date     string `json:"import_datetime"`
				Images   map[string]struct {
					Width  string `json:"width"`
					Height string `json:"height"`
					Size   string `json:"mp4_size"`
				} `json:"images"`
			} `json:"data"`
		}
		v := url.Values{"api_key": {key}, "q": {q.Text}, "limit": {strconv.Itoa(limit(q, 25))},
			"offset": {strconv.Itoa(q.Offset)}}
		err := getJSON(ctx, "https://api.giphy.com/v1/gifs/search?"+v.Encode(), &doc)
		if err == nil {
			var out []Item
			for _, d := range doc.Data {
				it := giphyItem(d.ID, d.Title, d.Username)
				it.Date, _, _ = strings.Cut(d.Date, " ")
				o := d.Images["original"]
				it.Width, _ = strconv.Atoi(o.Width)
				it.Height, _ = strconv.Atoi(o.Height)
				it.Size, _ = strconv.ParseInt(o.Size, 10, 64)
				out = append(out, it)
			}
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if q.Offset > 0 {
		return nil, nil // the page lists one batch; more needs an API key
	}
	page, err := get(ctx, "https://giphy.com/search/"+url.PathEscape(strings.Join(strings.Fields(q.Text), "-")))
	if err != nil {
		return nil, err
	}
	return parseGiphyHTML(string(page)), nil
}

// --- Tenor -----------------------------------------------------------------------

var tenorRe = regexp.MustCompile(`"mp4":\{"url":"(https://media1?\.tenor\.com/(?:m/)?([A-Za-z0-9_-]+)/([^"/]+)\.mp4)","duration":([0-9.]+),"preview":"[^"]*","dims":\[(\d+),(\d+)\],"size":(\d+)`)

func parseTenorHTML(page string) []Item {
	// The page embeds its data as JSON with every slash written as a
	// \u escape (or as \/); undo that before matching.
	bs := string(rune(92))
	page = strings.NewReplacer(bs+"u002F", "/", bs+"u002f", "/", bs+"/", "/").Replace(page)
	var out []Item
	seen := map[string]bool{}
	for _, m := range tenorRe.FindAllStringSubmatch(page, -1) {
		if seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		it := Item{Provider: "Tenor", ID: m[2], Kind: "gif", Title: strings.ReplaceAll(m[3], "-", " "), URL: m[1],
			Direct: true, Ext: ".mp4", Thumb: m[1]}
		it.Duration, _ = strconv.ParseFloat(m[4], 64)
		it.Width, _ = strconv.Atoi(m[5])
		it.Height, _ = strconv.Atoi(m[6])
		it.Size, _ = strconv.ParseInt(m[7], 10, 64)
		out = append(out, it)
	}
	return out
}

func searchTenor(ctx context.Context, q Query, k Keys) ([]Item, error) {
	if q.Offset > 0 {
		return nil, nil
	}
	slug := url.PathEscape(strings.Join(strings.Fields(q.Text), "-"))
	page, err := get(ctx, "https://tenor.com/search/"+slug+"-gifs")
	if err != nil {
		return nil, err
	}
	return parseTenorHTML(string(page)), nil
}

// --- Pinterest ---------------------------------------------------------------------

// parseGalleryDL reads gallery-dl's -j output: a list of [type, url, meta]
// entries, of which type 3 are the files.
func parseGalleryDL(data []byte, provider string) ([]Item, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("gallery-dl sent something that is not JSON")
	}
	var out []Item
	for _, r := range raw {
		var ent []json.RawMessage
		var typ int
		if json.Unmarshal(r, &ent) != nil || len(ent) < 3 || json.Unmarshal(ent[0], &typ) != nil || typ != 3 {
			continue
		}
		var u string
		var m struct {
			ID          json.Number `json:"id"`
			Title       string      `json:"title"`
			GridTitle   string      `json:"grid_title"`
			AltText     string      `json:"auto_alt_text"`
			Description string      `json:"description"`
			Extension   string      `json:"extension"`
			Width       int         `json:"width"`
			Height      int         `json:"height"`
			CreatedAt   string      `json:"created_at"`
			Pinner      struct {
				Username string `json:"username"`
			} `json:"pinner"`
			Images map[string]struct {
				URL string `json:"url"`
			} `json:"images"`
		}
		if json.Unmarshal(ent[1], &u) != nil || json.Unmarshal(ent[2], &m) != nil || u == "" {
			continue
		}
		it := Item{Provider: provider, ID: m.ID.String(), Author: m.Pinner.Username, Width: m.Width, Height: m.Height,
			Desc: strings.TrimSpace(m.Description)}
		for _, t := range []string{m.GridTitle, m.Title, m.AltText, m.Description} {
			if it.Title = strings.TrimSpace(t); it.Title != "" {
				break
			}
		}
		if it.Title == "" {
			it.Title = "pin " + it.ID
		}
		if f := strings.Fields(m.CreatedAt); len(f) >= 4 {
			it.Date = strings.Join(f[1:4], " ")
		}
		for _, size := range []string{"236x", "474x", "170x", "orig"} {
			if it.Thumb = m.Images[size].URL; it.Thumb != "" {
				break
			}
		}
		if s, ok := strings.CutPrefix(u, "ytdl:"); ok {
			// A video pin: an HLS playlist that ffmpeg and yt-dlp both read.
			it.URL, it.Kind, it.Ext = s, "video", ".m3u8"
		} else {
			it.URL, it.Direct, it.Ext = u, true, extOf(u)
			it.Kind = directExts[it.Ext]
			if it.Kind == "" {
				it.Kind = "image"
			}
			if it.Thumb == "" {
				it.Thumb = u
			}
		}
		out = append(out, it)
	}
	return out, nil
}

func searchPinterest(ctx context.Context, q Query, k Keys) ([]Item, error) {
	n := limit(q, 20)
	text := q.Text
	want := n
	switch q.Kind {
	case "gif":
		text += " gif"
		want = n * 2
	case "video":
		text += " video"
		want = n * 2
	}
	src := text
	if !IsURL(src) {
		src = "https://www.pinterest.com/search/pins/?q=" + url.QueryEscape(text)
	}
	out, err := run(ctx, "gallery-dl", "-j", "--range", fmt.Sprintf("%d-%d", q.Offset+1, q.Offset+want), src)
	if err != nil {
		return nil, err
	}
	items, err := parseGalleryDL(out, "Pinterest")
	if err != nil {
		return nil, err
	}
	items = keep(items, Query{Kind: q.Kind})
	if len(items) > n {
		items = items[:n]
	}
	return items, nil
}

// --- Internet Archive -------------------------------------------------------------

func parseArchive(data []byte) ([]Item, error) {
	var doc struct {
		Response struct {
			Docs []struct {
				Identifier string          `json:"identifier"`
				Title      json.RawMessage `json:"title"`
				Creator    json.RawMessage `json:"creator"`
				Downloads  int64           `json:"downloads"`
				Year       json.RawMessage `json:"year"`
				Desc       json.RawMessage `json:"description"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("archive.org sent something that is not JSON")
	}
	// Fields are a string, a number or a list of either, depending on the item.
	str := func(r json.RawMessage) string {
		var s string
		var l []any
		var n json.Number
		switch {
		case json.Unmarshal(r, &s) == nil:
			return s
		case json.Unmarshal(r, &n) == nil:
			return n.String()
		case json.Unmarshal(r, &l) == nil && len(l) > 0:
			return fmt.Sprint(l[0])
		}
		return ""
	}
	var out []Item
	for _, d := range doc.Response.Docs {
		if d.Identifier == "" {
			continue
		}
		title := str(d.Title)
		if title == "" {
			title = d.Identifier
		}
		out = append(out, Item{Provider: "Archive", ID: d.Identifier, Kind: "video", Title: title, Author: str(d.Creator),
			URL: "https://archive.org/details/" + d.Identifier, Thumb: "https://archive.org/services/img/" + d.Identifier,
			Views: d.Downloads, Date: str(d.Year), Desc: stripTags(str(d.Desc))})
	}
	return out, nil
}

func searchArchive(ctx context.Context, q Query, k Keys) ([]Item, error) {
	n := limit(q, 20)
	v := url.Values{"q": {"(" + q.Text + ") AND mediatype:movies AND NOT collection:(tvnews OR tvarchive)"},
		"fl[]": {"identifier", "title", "creator", "downloads", "year", "description"}, "rows": {strconv.Itoa(n)},
		"page": {strconv.Itoa(q.Offset/n + 1)}, "output": {"json"}}
	// The archive's own relevance ranking buries the well-known items, so
	// the most downloaded matches come first unless a date order is asked for.
	v.Set("sort[]", "downloads desc")
	if q.Sort == "date" {
		v.Set("sort[]", "publicdate desc")
	}
	body, err := get(ctx, "https://archive.org/advancedsearch.php?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return parseArchive(body)
}

// --- Wikimedia Commons -------------------------------------------------------------

func parseWikimedia(data []byte) ([]Item, error) {
	var doc struct {
		Query struct {
			Pages map[string]struct {
				Title     string `json:"title"`
				Index     int    `json:"index"`
				ImageInfo []struct {
					URL      string  `json:"url"`
					ThumbURL string  `json:"thumburl"`
					Size     int64   `json:"size"`
					Width    int     `json:"width"`
					Height   int     `json:"height"`
					Duration float64 `json:"duration"`
					Mime     string  `json:"mime"`
					User     string  `json:"user"`
					Time     string  `json:"timestamp"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("Wikimedia sent something that is not JSON")
	}
	type ranked struct {
		idx int
		it  Item
	}
	var rs []ranked
	for id, p := range doc.Query.Pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		u, _, _ := strings.Cut(ii.URL, "?")
		it := Item{Provider: "Wikimedia", ID: id, Title: strings.TrimPrefix(p.Title, "File:"), Author: ii.User, URL: u,
			Direct: true, Ext: extOf(u), Thumb: ii.ThumbURL, Size: ii.Size, Width: ii.Width, Height: ii.Height,
			Duration: ii.Duration}
		it.Date, _, _ = strings.Cut(ii.Time, "T")
		switch {
		case ii.Mime == "image/gif":
			it.Kind = "gif"
		case strings.HasPrefix(ii.Mime, "image/"):
			it.Kind = "image"
		case strings.HasPrefix(ii.Mime, "audio/"):
			it.Kind = "audio"
		default:
			it.Kind = "video"
		}
		rs = append(rs, ranked{p.Index, it})
	}
	// The API returns a map; put the results back in ranking order.
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].idx < rs[j-1].idx; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
	out := make([]Item, len(rs))
	for i, r := range rs {
		out[i] = r.it
	}
	return out, nil
}

func searchWikimedia(ctx context.Context, q Query, k Keys) ([]Item, error) {
	filter := "filetype:video"
	switch q.Kind {
	case "gif":
		filter = "filemime:image/gif"
	case "image":
		filter = "filetype:bitmap"
	case "any":
		filter = "filetype:video|bitmap"
	}
	v := url.Values{"action": {"query"}, "generator": {"search"}, "gsrsearch": {filter + " " + q.Text},
		"gsrnamespace": {"6"}, "gsrlimit": {strconv.Itoa(limit(q, 20))}, "gsroffset": {strconv.Itoa(q.Offset)},
		"prop": {"imageinfo"}, "iiprop": {"url|size|mime|user|timestamp"}, "iiurlwidth": {"320"}, "format": {"json"}}
	body, err := get(ctx, "https://commons.wikimedia.org/w/api.php?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return parseWikimedia(body)
}

// --- any link ------------------------------------------------------------------------

func searchURL(ctx context.Context, q Query, k Keys) ([]Item, error) {
	u := strings.TrimSpace(q.Text)
	if !IsURL(u) {
		return nil, errors.New("paste a full link, starting with http:// or https://")
	}
	if q.Offset > 0 {
		return nil, nil
	}
	it := Item{Provider: "URL", ID: u, Title: u, URL: u, Kind: "video"}
	if kind, ok := directExts[extOf(u)]; ok {
		it.Direct, it.Kind, it.Ext = true, kind, extOf(u)
		return []Item{it}, nil
	}
	// A page: ask yt-dlp what is behind it. A playlist gives several results.
	out, err := run(ctx, "yt-dlp", "--flat-playlist", "-J", "--no-warnings", "--playlist-items", "1-50", u)
	if err != nil {
		return nil, err
	}
	if items, err := parseDLPSearch(out, "URL"); err == nil && len(items) > 0 {
		return items, nil
	}
	var e dlpEntry
	if json.Unmarshal(out, &e) == nil && e.Title != "" {
		it.Title, it.Author, it.Duration, it.Views, it.Thumb, it.Desc = e.Title, e.Uploader, e.Duration, e.ViewCount, e.Thumbnail, e.Desc
		if e.Channel != "" {
			it.Author = e.Channel
		}
	}
	return []Item{it}, nil
}
