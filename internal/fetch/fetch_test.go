package fetch

import (
	"strings"
	"testing"
)

// The parsers are tested against trimmed copies of what each site really
// sends; nothing here touches the network.

func TestParseYouTubeSearch(t *testing.T) {
	items, err := parseYouTubeSearch([]byte(`{"items":[
	 {"id":{"kind":"youtube#video","videoId":"abc123"},"snippet":{"publishedAt":"2024-05-06T07:08:09Z",
	  "title":"Tom &amp; Jerry &#39;best of&#39;","channelTitle":"Cartoons","description":"d",
	  "thumbnails":{"medium":{"url":"https://i.ytimg.com/vi/abc123/mqdefault.jpg"}}}},
	 {"id":{"kind":"youtube#channel"},"snippet":{"title":"a channel"}}]}`))
	if err != nil || len(items) != 1 {
		t.Fatalf("items %v err %v", items, err)
	}
	it := items[0]
	if it.Title != "Tom & Jerry 'best of'" || it.Author != "Cartoons" || it.Date != "2024-05-06" ||
		it.URL != "https://www.youtube.com/watch?v=abc123" || it.Kind != "video" || it.Direct {
		t.Errorf("item: %+v", it)
	}
	_, err = parseYouTubeSearch([]byte(`{"error":{"code":403,"message":"The request cannot be completed because you have exceeded your <a href=\"/youtube/v3/getting-started#quota\">quota</a>."}}`))
	if err == nil || !strings.Contains(err.Error(), "exceeded your quota") || strings.Contains(err.Error(), "<a") {
		t.Errorf("quota error: %v", err)
	}
}

func TestISODuration(t *testing.T) {
	cases := map[string]float64{"PT1H2M3S": 3723, "PT45S": 45, "PT10M": 600, "P1DT1S": 86401, "PT0S": 0, "junk": 0}
	for in, want := range cases {
		if got := isoDuration(in); got != want {
			t.Errorf("isoDuration(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDLPSearch(t *testing.T) {
	items, err := parseDLPSearch([]byte(`{"entries":[
	 {"id":"I3zr","title":"Cat Clapping","url":"https://www.youtube.com/watch?v=I3zr","duration":62.0,
	  "view_count":16300,"channel":"V0LTRIX","thumbnails":[{"url":"https://i.ytimg.com/vi/I3zr/hq720.jpg"}]},
	 {"id":"x","title":"","url":"https://example.com"},
	 {"id":"y","title":"Other site","webpage_url":"https://vimeo.com/1","uploader":"u","upload_date":"20230102",
	  "thumbnail":"https://t/1.jpg"}]}`), "YouTube")
	if err != nil || len(items) != 2 {
		t.Fatalf("items %v err %v", items, err)
	}
	if items[0].Duration != 62 || items[0].Views != 16300 || items[0].Author != "V0LTRIX" ||
		items[0].Thumb != "https://i.ytimg.com/vi/I3zr/mqdefault.jpg" {
		t.Errorf("first: %+v", items[0])
	}
	if items[1].URL != "https://vimeo.com/1" || items[1].Author != "u" || items[1].Date != "2023-01-02" {
		t.Errorf("second: %+v", items[1])
	}
}

func TestParseGiphyHTML(t *testing.T) {
	page := `<a href="https://giphy.com/gifs/bailando-ai-dance-2banana1-tphCApwvdtC1VJabZ1">x</a>
	  "url":"https://giphy.com/gifs/cat-happy-mkl-A0Zt7yuDULiy4ofmVD?x" again giphy.com/gifs/cat-happy-mkl-A0Zt7yuDULiy4ofmVD"
	  <img src="https://media1.giphy.com/media/v1.Y2lk/tphCApwvdtC1VJabZ1/giphy.webp">`
	items := parseGiphyHTML(page)
	if len(items) != 2 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	if items[0].ID != "tphCApwvdtC1VJabZ1" || items[0].Title != "bailando ai dance 2banana1" || !items[0].Direct ||
		items[0].URL != "https://i.giphy.com/media/tphCApwvdtC1VJabZ1/giphy.mp4" || items[0].Kind != "gif" {
		t.Errorf("first: %+v", items[0])
	}
}

func TestParseTenorHTML(t *testing.T) {
	page := `"tinygif":{"url":"https:\u002F\u002Fmedia.tenor.com\u002FaGjAAAAM\u002Fcat-cat-dance.gif","duration":1.6,"preview":"","dims":[220,211],"size":303516},` +
		`"mp4":{"url":"https:\u002F\u002Fmedia.tenor.com\u002FaGj-frNYMFEAAAPo\u002Fcat-cat-dance.mp4","duration":1.6,"preview":"","dims":[498,476],"size":82396},` +
		`"mp4":{"url":"https:\u002F\u002Fmedia.tenor.com\u002FaGj-frNYMFEAAAPo\u002Fcat-cat-dance.mp4","duration":1.6,"preview":"","dims":[498,476],"size":82396}`
	items := parseTenorHTML(page)
	if len(items) != 1 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	it := items[0]
	if it.Title != "cat cat dance" || it.Width != 498 || it.Height != 476 || it.Size != 82396 || it.Duration != 1.6 ||
		it.URL != "https://media.tenor.com/aGj-frNYMFEAAAPo/cat-cat-dance.mp4" || it.Ext != ".mp4" {
		t.Errorf("item: %+v", it)
	}
}

func TestParseGalleryDL(t *testing.T) {
	data := `[[2,{"board":{}}],
	 [3,"https://i.pinimg.com/originals/17/b8/99/17b8.jpg",{"id":"1062779","grid_title":"","title":" ",
	   "auto_alt_text":"a close up of a cat","extension":"jpg","width":320,"height":469,
	   "created_at":"Sun, 20 Sep 2026 15:33:47 +0000","pinner":{"username":"sona"},
	   "images":{"236x":{"url":"https://i.pinimg.com/236x/17/b8/99/17b8.jpg"},"orig":{"url":"https://i.pinimg.com/originals/17/b8/99/17b8.jpg"}}}],
	 [3,"ytdl:https://v1.pinimg.com/videos/iht/hls/8e/b6/17/8eb6.m3u8",{"id":1407443631282393,"grid_title":"Scubaa","extension":"mp4"}],
	 [3,"https://i.pinimg.com/originals/aa/bb/cc/x.gif",{"id":"7","title":"dancing"}]]`
	items, err := parseGalleryDL([]byte(data), "Pinterest")
	if err != nil || len(items) != 3 {
		t.Fatalf("items %+v err %v", items, err)
	}
	if it := items[0]; it.Title != "a close up of a cat" || it.Kind != "image" || !it.Direct || it.Author != "sona" ||
		it.Thumb != "https://i.pinimg.com/236x/17/b8/99/17b8.jpg" || it.Date != "20 Sep 2026" || it.Height != 469 {
		t.Errorf("image pin: %+v", it)
	}
	if it := items[1]; it.Kind != "video" || it.Direct || it.Ext != ".m3u8" || it.ID != "1407443631282393" ||
		it.URL != "https://v1.pinimg.com/videos/iht/hls/8e/b6/17/8eb6.m3u8" {
		t.Errorf("video pin: %+v", it)
	}
	if it := items[2]; it.Kind != "gif" || it.Thumb != it.URL {
		t.Errorf("gif pin: %+v", it)
	}
}

func TestParseArchive(t *testing.T) {
	items, err := parseArchive([]byte(`{"response":{"docs":[
	 {"identifier":"night_of_the_living_dead","title":"Night of the Living Dead","creator":["George A. Romero","x"],
	  "downloads":3500000,"year":1968,"description":"<p>A <b>classic</b>.</p>"},
	 {"identifier":"untitled_item","year":"2001"}]}}`))
	if err != nil || len(items) != 2 {
		t.Fatalf("items %+v err %v", items, err)
	}
	if it := items[0]; it.Author != "George A. Romero" || it.Date != "1968" || it.Views != 3500000 ||
		it.Desc != "A classic." || it.URL != "https://archive.org/details/night_of_the_living_dead" {
		t.Errorf("first: %+v", it)
	}
	if items[1].Title != "untitled_item" || items[1].Date != "2001" {
		t.Errorf("second: %+v", items[1])
	}
}

func TestParseWikimedia(t *testing.T) {
	items, err := parseWikimedia([]byte(`{"query":{"pages":{
	 "2":{"title":"File:B.gif","index":2,"imageinfo":[{"url":"https://upload.wikimedia.org/b/B.gif","mime":"image/gif","size":10}]},
	 "1":{"title":"File:Cat birth 2.ogv","index":1,"imageinfo":[{"size":3443359,"width":320,"height":240,
	   "duration":40.13,"thumburl":"https://thumb/x.jpg","url":"https://upload.wikimedia.org/b/b6/Cat_birth_2.ogv?utm_source=x",
	   "mime":"application/ogg","user":"Someone","timestamp":"2007-01-02T03:04:05Z"}]},
	 "3":{"title":"File:missing","index":3}}}}`))
	if err != nil || len(items) != 2 {
		t.Fatalf("items %+v err %v", items, err)
	}
	if it := items[0]; it.Title != "Cat birth 2.ogv" || it.Kind != "video" || it.Ext != ".ogv" || it.Duration != 40.13 ||
		it.URL != "https://upload.wikimedia.org/b/b6/Cat_birth_2.ogv" || it.Date != "2007-01-02" {
		t.Errorf("results must come back in ranking order, video first: %+v", it)
	}
	if items[1].Kind != "gif" {
		t.Errorf("second: %+v", items[1])
	}
}

func TestParseDLPInfo(t *testing.T) {
	d, err := parseDLPInfo([]byte(`{"title":"T","channel":"C","uploader":"U","upload_date":"20240102","duration":3723,
	 "view_count":1234567,"like_count":0,"description":" hello ","extractor_key":"Youtube","width":1920,"height":1080,"fps":30,
	 "formats":[
	  {"format_id":"sb0","ext":"mhtml","format_note":"storyboard","protocol":"mhtml"},
	  {"format_id":"140","ext":"m4a","vcodec":"none","acodec":"mp4a.40.2","tbr":129.5,"filesize":1048576},
	  {"format_id":"137","ext":"mp4","width":1920,"height":1080,"fps":30,"vcodec":"avc1.640028","acodec":"none","tbr":4400,"filesize_approx":52428800},
	  {"format_id":"18","ext":"mp4","width":640,"height":360,"fps":30,"vcodec":"avc1.42001E","acodec":"mp4a.40.2","tbr":500}]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range d.Lines {
		got[l[0]] = l[1]
	}
	if got["By"] != "C" || got["Uploaded"] != "2024-01-02" || got["Length"] != "1:02:03" || got["Views"] != "1.2M" ||
		got["Best picture"] != "1920×1080 at 30 fps" || got["Likes"] != "" || d.Desc != "hello" {
		t.Errorf("lines: %v desc %q", got, d.Desc)
	}
	if len(d.Formats) != 3 || !strings.Contains(d.Formats[0], "audio only") || !strings.Contains(d.Formats[0], "1.0 MiB") ||
		!strings.Contains(d.Formats[1], "1920x1080@30") || !strings.Contains(d.Formats[1], "video only") ||
		!strings.Contains(d.Formats[2], "avc1+mp4a") {
		t.Errorf("formats: %q", d.Formats)
	}
}

func TestKeepFilters(t *testing.T) {
	items := []Item{{ID: "short", Duration: 30, Kind: "video"}, {ID: "mid", Duration: 600, Kind: "video"},
		{ID: "long", Duration: 4000, Kind: "video"}, {ID: "gif", Kind: "gif"}, {ID: "unknown"}}
	ids := func(q Query) string {
		var out []string
		for _, it := range keep(append([]Item(nil), items...), q) {
			out = append(out, it.ID)
		}
		return strings.Join(out, ",")
	}
	if got := ids(Query{Duration: "short"}); got != "short,gif,unknown" {
		t.Errorf("short: %s", got)
	}
	if got := ids(Query{Duration: "long"}); got != "long,gif,unknown" {
		t.Errorf("long: %s", got)
	}
	if got := ids(Query{Kind: "gif"}); got != "gif,unknown" {
		t.Errorf("gif: %s", got)
	}
	if got := ids(Query{Kind: "any", Duration: "any"}); got != "short,mid,long,gif,unknown" {
		t.Errorf("any: %s", got)
	}
}

func TestNamesAndFormats(t *testing.T) {
	cases := map[string]string{
		"AC/DC: Back <in> Black?": "AC DC Back in Black",
		"  ..weird\x00name..  ":   "weird name",
		"":                        "download",
		strings.Repeat("ż", 200):  strings.Repeat("ż", 90),
		"a\\b|c*d\"e":             "a b c d e",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"https://x.com/a/b.MP4?x=1": ".mp4", "https://x.com/watch?v=1.5": "",
		"https://x.com/a.b/c": "", "https://x/y.m3u8": ".m3u8"} {
		if got := extOf(in); got != want {
			t.Errorf("extOf(%q) = %q, want %q", in, got, want)
		}
	}
	seen := map[string]bool{}
	for _, f := range Formats {
		if seen[f.ID] || f.Label == "" {
			t.Errorf("format %q is duplicated or unlabeled", f.ID)
		}
		seen[f.ID] = true
		if f.Ext != "" && len(convertArgs(f)) == 0 {
			t.Errorf("format %q has no ffmpeg options", f.ID)
		}
		if len(dlpArgs(f)) < 2 {
			t.Errorf("format %q has no yt-dlp options", f.ID)
		}
	}
	if a := strings.Join(dlpArgs(Format{Ext: "mp4", Height: 720}), " "); !strings.Contains(a, "[height<=720]") {
		t.Errorf("height limit missing: %s", a)
	}
	if Count(1500) != "1.5k" || Count(0) != "" || Clock(3723) != "1:02:03" || Clock(59.6) != "1:00" {
		t.Error("Count / Clock formatting is off")
	}
}
