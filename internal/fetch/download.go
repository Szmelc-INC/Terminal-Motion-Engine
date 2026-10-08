package fetch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Stream is what the player needs to play an item without downloading it.
type Stream struct {
	Video string // URL with the picture (and usually the sound)
	Audio string // separate sound URL when the site serves them apart
}

// Resolve finds the stream URLs of an item. For pages it asks yt-dlp for a
// picture of at most maxHeight lines, which is plenty for a terminal.
func Resolve(ctx context.Context, it Item, maxHeight int) (Stream, error) {
	if it.Direct || it.Ext == ".m3u8" {
		return Stream{Video: it.URL}, nil
	}
	if maxHeight <= 0 {
		maxHeight = 480
	}
	h := strconv.Itoa(maxHeight)
	sel := "bv*[height<=" + h + "][vcodec^=avc1]+ba[ext=m4a]/bv*[height<=" + h + "]+ba/b[height<=" + h + "]/b"
	out, err := run(ctx, "yt-dlp", "-g", "--no-playlist", "--no-warnings", "-f", sel, it.URL)
	if err != nil {
		return Stream{}, err
	}
	var urls []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); IsURL(l) {
			urls = append(urls, l)
		}
	}
	switch len(urls) {
	case 0:
		return Stream{}, errors.New("yt-dlp found no playable stream")
	case 1:
		return Stream{Video: urls[0]}, nil
	}
	return Stream{Video: urls[0], Audio: urls[1]}, nil
}

// Detail is the extended information about an item.
type Detail struct {
	Lines   [][2]string // label, value
	Formats []string    // one line per available stream
	Desc    string
}

func human(n int64) string {
	switch {
	case n <= 0:
		return ""
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// Count formats a view or download count: 1234567 -> "1.2M".
func Count(n int64) string {
	switch {
	case n <= 0:
		return ""
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return strconv.FormatInt(n, 10)
}

// Clock formats seconds as m:ss or h:mm:ss.
func Clock(sec float64) string {
	if sec <= 0 {
		return ""
	}
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

type dlpInfo struct {
	Title       string   `json:"title"`
	Uploader    string   `json:"uploader"`
	Channel     string   `json:"channel"`
	UploadDate  string   `json:"upload_date"`
	Duration    float64  `json:"duration"`
	ViewCount   int64    `json:"view_count"`
	LikeCount   int64    `json:"like_count"`
	Comments    int64    `json:"comment_count"`
	Description string   `json:"description"`
	Extractor   string   `json:"extractor_key"`
	License     string   `json:"license"`
	AgeLimit    int      `json:"age_limit"`
	Live        string   `json:"live_status"`
	Categories  []string `json:"categories"`
	Tags        []string `json:"tags"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	FPS         float64  `json:"fps"`
	Formats     []struct {
		ID       string  `json:"format_id"`
		Ext      string  `json:"ext"`
		Note     string  `json:"format_note"`
		Width    int     `json:"width"`
		Height   int     `json:"height"`
		FPS      float64 `json:"fps"`
		VCodec   string  `json:"vcodec"`
		ACodec   string  `json:"acodec"`
		TBR      float64 `json:"tbr"`
		Size     int64   `json:"filesize"`
		SizeEst  int64   `json:"filesize_approx"`
		Protocol string  `json:"protocol"`
	} `json:"formats"`
}

func parseDLPInfo(data []byte) (Detail, error) {
	var in dlpInfo
	if err := json.Unmarshal(data, &in); err != nil {
		return Detail{}, errors.New("yt-dlp sent something that is not JSON")
	}
	var d Detail
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" && v != "0" {
			d.Lines = append(d.Lines, [2]string{k, v})
		}
	}
	add("Title", in.Title)
	who := in.Channel
	if who == "" {
		who = in.Uploader
	}
	add("By", who)
	add("Site", in.Extractor)
	if len(in.UploadDate) == 8 {
		add("Uploaded", in.UploadDate[:4]+"-"+in.UploadDate[4:6]+"-"+in.UploadDate[6:])
	}
	add("Length", Clock(in.Duration))
	if in.Width > 0 {
		res := fmt.Sprintf("%d×%d", in.Width, in.Height)
		if in.FPS > 0 {
			res += fmt.Sprintf(" at %.4g fps", in.FPS)
		}
		add("Best picture", res)
	}
	add("Views", Count(in.ViewCount))
	add("Likes", Count(in.LikeCount))
	add("Comments", Count(in.Comments))
	add("Categories", strings.Join(in.Categories, ", "))
	if len(in.Tags) > 12 {
		in.Tags = in.Tags[:12]
	}
	add("Tags", strings.Join(in.Tags, ", "))
	add("License", in.License)
	if in.AgeLimit > 0 {
		add("Age limit", strconv.Itoa(in.AgeLimit)+"+")
	}
	if in.Live != "" && in.Live != "not_live" {
		add("Live", strings.ReplaceAll(in.Live, "_", " "))
	}
	d.Desc = strings.TrimSpace(in.Description)
	for _, f := range in.Formats {
		if strings.Contains(f.Note, "storyboard") || f.Protocol == "mhtml" {
			continue
		}
		kind := "video+audio"
		switch {
		case f.VCodec == "none" || f.VCodec == "":
			kind = "audio only"
		case f.ACodec == "none" || f.ACodec == "":
			kind = "video only"
		}
		res := "—"
		if f.Height > 0 {
			res = fmt.Sprintf("%dx%d", f.Width, f.Height)
			if f.FPS > 0 {
				res += fmt.Sprintf("@%.0f", f.FPS)
			}
		}
		size := f.Size
		if size == 0 {
			size = f.SizeEst
		}
		codec := strings.TrimSuffix(strings.Join([]string{short(f.VCodec), short(f.ACodec)}, "+"), "+")
		d.Formats = append(d.Formats, fmt.Sprintf("%-6s %-5s %-13s %-11s %-14s %6.0fk %9s", f.ID, f.Ext, res, kind,
			strings.TrimPrefix(codec, "+"), f.TBR, human(size)))
	}
	return d, nil
}

func short(codec string) string {
	if codec == "none" || codec == "" {
		return ""
	}
	c, _, _ := strings.Cut(codec, ".")
	return c
}

// Info collects everything that can be found out about an item: yt-dlp's
// metadata for pages, ffprobe's for files.
func Info(ctx context.Context, it Item) (Detail, error) {
	if !it.Direct && it.Ext != ".m3u8" {
		out, err := run(ctx, "yt-dlp", "-J", "--no-playlist", "--no-warnings", it.URL)
		if err != nil {
			return Detail{}, err
		}
		return parseDLPInfo(out)
	}
	var d Detail
	add := func(k, v string) {
		if v != "" {
			d.Lines = append(d.Lines, [2]string{k, v})
		}
	}
	add("Title", it.Title)
	add("By", it.Author)
	add("Site", it.Provider)
	add("Date", it.Date)
	add("Address", it.URL)
	out, err := run(ctx, "ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", it.URL)
	if err != nil {
		return d, err
	}
	var doc struct {
		Streams []struct {
			Type     string `json:"codec_type"`
			Codec    string `json:"codec_name"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			Rate     string `json:"avg_frame_rate"`
			Channels int    `json:"channels"`
			SRate    string `json:"sample_rate"`
			BitRate  string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			Name     string `json:"format_long_name"`
			Duration string `json:"duration"`
			Size     string `json:"size"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return d, nil
	}
	add("Container", doc.Format.Name)
	if sec, _ := strconv.ParseFloat(doc.Format.Duration, 64); sec > 0 {
		add("Length", Clock(sec))
	}
	if n, _ := strconv.ParseInt(doc.Format.Size, 10, 64); n > 0 {
		add("Size", human(n))
	}
	if n, _ := strconv.ParseFloat(doc.Format.BitRate, 64); n > 0 {
		add("Bit rate", fmt.Sprintf("%.0f kbit/s", n/1000))
	}
	for _, s := range doc.Streams {
		switch s.Type {
		case "video":
			line := fmt.Sprintf("video  %-8s %dx%d", s.Codec, s.Width, s.Height)
			if a, b, ok := strings.Cut(s.Rate, "/"); ok {
				n, _ := strconv.ParseFloat(a, 64)
				if den, _ := strconv.ParseFloat(b, 64); den > 0 && n > 0 {
					line += fmt.Sprintf("  %.4g fps", n/den)
				}
			}
			d.Formats = append(d.Formats, line)
		case "audio":
			d.Formats = append(d.Formats, fmt.Sprintf("audio  %-8s %d ch  %s Hz", s.Codec, s.Channels, s.SRate))
		}
	}
	return d, nil
}

// Format is a download target.
type Format struct {
	ID, Label string
	Ext       string // "" keeps whatever the source is
	Height    int    // picture height limit, 0 = none
	Audio     bool
}

// Formats lists what a download can be saved as.
var Formats = []Format{
	{ID: "mp4", Label: "MP4 video — best quality", Ext: "mp4"},
	{ID: "mp4-1080", Label: "MP4 video — up to 1080p", Ext: "mp4", Height: 1080},
	{ID: "mp4-720", Label: "MP4 video — up to 720p", Ext: "mp4", Height: 720},
	{ID: "mp4-480", Label: "MP4 video — up to 480p (small, plenty for a terminal)", Ext: "mp4", Height: 480},
	{ID: "mp4-240", Label: "MP4 video — up to 240p (tiny)", Ext: "mp4", Height: 240},
	{ID: "webm", Label: "WebM video (VP9 + Opus)", Ext: "webm"},
	{ID: "mkv", Label: "MKV — best streams, no re-encoding", Ext: "mkv"},
	{ID: "gif", Label: "Animated GIF (480 px wide, 15 fps)", Ext: "gif"},
	{ID: "mp3", Label: "MP3 audio", Ext: "mp3", Audio: true},
	{ID: "m4a", Label: "M4A audio (AAC)", Ext: "m4a", Audio: true},
	{ID: "opus", Label: "Opus audio", Ext: "opus", Audio: true},
	{ID: "flac", Label: "FLAC audio (lossless container)", Ext: "flac", Audio: true},
	{ID: "wav", Label: "WAV audio (uncompressed)", Ext: "wav", Audio: true},
	{ID: "original", Label: "Original file, exactly as served"},
}

// FindFormat looks a format up by its ID.
func FindFormat(id string) (Format, bool) {
	for _, f := range Formats {
		if f.ID == id {
			return f, true
		}
	}
	return Format{}, false
}

// Progress is reported while a download runs.
type Progress struct {
	Stage   string  // "downloading", "converting"
	Percent float64 // 0..100, -1 when unknown
	Speed   string
	ETA     string
}

var unsafeName = regexp.MustCompile(`[/\\\x00-\x1f<>:"|?*]+`)

// SafeName turns a title into a file name.
func SafeName(title string) string {
	s := strings.TrimSpace(unsafeName.ReplaceAllString(title, " "))
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, ". ")
	if r := []rune(s); len(r) > 90 {
		s = strings.TrimSpace(string(r[:90]))
	}
	if s == "" {
		s = "download"
	}
	return s
}

// freePath returns dir/name.ext, adding a number when that file exists.
func freePath(dir, name, ext string) string {
	p := filepath.Join(dir, name+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(p); err != nil {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
	}
}

// Download saves an item into dir in the given format and returns the path
// of the finished file.
func Download(ctx context.Context, it Item, f Format, dir string, report func(Progress)) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if report == nil {
		report = func(Progress) {}
	}
	if !it.Direct {
		return downloadPage(ctx, it, f, dir, report)
	}
	ext := it.Ext
	if ext == "" {
		ext = ".bin"
	}
	name := SafeName(it.Title)
	if f.Ext == "" || "."+f.Ext == ext {
		path := freePath(dir, name, ext)
		return path, httpDownload(ctx, it.URL, path, report)
	}
	tmp := freePath(dir, "."+name+".part", ext)
	defer os.Remove(tmp)
	if err := httpDownload(ctx, it.URL, tmp, report); err != nil {
		return "", err
	}
	path := freePath(dir, name, "."+f.Ext)
	if err := Convert(ctx, tmp, path, f, it.Duration, report); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func httpDownload(ctx context.Context, u, path string, report func(Progress)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &httpError{resp.StatusCode, hostOf(u)}
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	buf := make([]byte, 64<<10)
	var done int64
	start, last := time.Now(), time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(path)
				return werr
			}
			done += int64(n)
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				p := Progress{Stage: "downloading", Percent: -1}
				if el := time.Since(start).Seconds(); el > 0.3 {
					rate := float64(done) / el
					p.Speed = human(int64(rate)) + "/s"
					if resp.ContentLength > 0 && rate > 0 {
						p.ETA = Clock(float64(resp.ContentLength-done)/rate + 0.5)
					}
				}
				if resp.ContentLength > 0 {
					p.Percent = float64(done) / float64(resp.ContentLength) * 100
				}
				report(p)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(path)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return rerr
		}
	}
	return out.Close()
}

// dlpArgs builds the yt-dlp options that select and package a format.
func dlpArgs(f Format) []string {
	h := ""
	if f.Height > 0 {
		h = fmt.Sprintf("[height<=%d]", f.Height)
	}
	switch {
	case f.Audio:
		return []string{"-f", "ba/b", "-x", "--audio-format", f.Ext}
	case f.Ext == "mp4":
		// H.264 first: it plays everywhere and decodes fastest.
		return []string{"-f", "bv*" + h + "[vcodec^=avc1]+ba[ext=m4a]/bv*" + h + "[ext=mp4]+ba[ext=m4a]/b" + h +
			"[ext=mp4]/bv*" + h + "+ba/b" + h + "/b",
			"--merge-output-format", "mp4", "--remux-video", "mp4"}
	case f.Ext == "webm":
		return []string{"-f", "bv*[ext=webm]+ba[ext=webm]/b[ext=webm]/bv*+ba/b", "--recode-video", "webm"}
	case f.Ext == "mkv":
		return []string{"-f", "bv*+ba/b", "--merge-output-format", "mkv"}
	case f.Ext == "gif":
		return []string{"-f", "b[height<=480]/bv*[height<=480]+ba/bv*[height<=480]/b"}
	}
	return []string{"-f", "b/bv*+ba"}
}

var dlpProgressRe = regexp.MustCompile(`^TERMO\s+([0-9.]+)%\s+(\S*)\s+(\S*)`)

func downloadPage(ctx context.Context, it Item, f Format, dir string, report func(Progress)) (string, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", errors.New("yt-dlp is not installed")
	}
	args := []string{"--no-playlist", "--no-warnings", "--newline", "--no-simulate", "--no-mtime",
		"--progress-template", "download:TERMO %(progress._percent_str)s %(progress._speed_str)s %(progress._eta_str)s",
		"--print", "after_move:TERMOFILE %(filepath)s",
		"-P", dir}
	if it.Ext == ".m3u8" || it.Provider == "Pinterest" {
		// A bare stream has no title of its own; use the one from the search.
		args = append(args, "-o", strings.ReplaceAll(SafeName(it.Title), "%", "%%")+".%(ext)s")
	} else {
		args = append(args, "-o", "%(title).90B [%(id)s].%(ext)s")
	}
	args = append(args, dlpArgs(f)...)
	args = append(args, it.URL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return "", err
	}
	path := ""
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if p, ok := strings.CutPrefix(line, "TERMOFILE "); ok {
			path = strings.TrimSpace(p)
			continue
		}
		if m := dlpProgressRe.FindStringSubmatch(line); m != nil {
			pct, _ := strconv.ParseFloat(m[1], 64)
			report(Progress{Stage: "downloading", Percent: pct, Speed: strings.TrimSpace(m[2]), ETA: strings.TrimSpace(m[3])})
		} else if strings.HasPrefix(line, "[Merger]") || strings.HasPrefix(line, "[ExtractAudio]") ||
			strings.HasPrefix(line, "[VideoConvertor]") || strings.HasPrefix(line, "[VideoRemuxer]") {
			report(Progress{Stage: "converting", Percent: -1})
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("yt-dlp: " + lastLine(errBuf.String(), err.Error()))
	}
	if path == "" {
		return "", errors.New("yt-dlp finished without reporting a file")
	}
	if f.Ext == "gif" && !strings.HasSuffix(strings.ToLower(path), ".gif") {
		gif := freePath(dir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), ".gif")
		err := Convert(ctx, path, gif, f, it.Duration, report)
		os.Remove(path)
		if err != nil {
			os.Remove(gif)
			return "", err
		}
		path = gif
	}
	return path, nil
}

// convertArgs returns the ffmpeg output options for a target format.
func convertArgs(f Format) []string {
	scale := "scale=trunc(iw/2)*2:trunc(ih/2)*2"
	if f.Height > 0 {
		scale = fmt.Sprintf("scale=-2:'min(%d,trunc(ih/2)*2)'", f.Height)
	}
	switch f.Ext {
	case "mp4":
		return []string{"-vf", scale, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "160k", "-movflags", "+faststart"}
	case "webm":
		return []string{"-vf", scale, "-c:v", "libvpx-vp9", "-crf", "32", "-b:v", "0", "-row-mt", "1", "-cpu-used", "4",
			"-c:a", "libopus", "-b:a", "128k"}
	case "mkv":
		return []string{"-c", "copy"}
	case "gif":
		return []string{"-an", "-filter_complex",
			"fps=15,scale='min(480,iw)':-1:flags=lanczos,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4"}
	case "mp3":
		return []string{"-vn", "-c:a", "libmp3lame", "-q:a", "2"}
	case "m4a":
		return []string{"-vn", "-c:a", "aac", "-b:a", "192k"}
	case "opus":
		return []string{"-vn", "-c:a", "libopus", "-b:a", "128k"}
	case "flac":
		return []string{"-vn", "-c:a", "flac"}
	case "wav":
		return []string{"-vn", "-c:a", "pcm_s16le"}
	}
	return []string{"-c", "copy"}
}

// Convert re-encodes src into dst with ffmpeg. duration (seconds, 0 if
// unknown) is only used to report progress.
func Convert(ctx context.Context, src, dst string, f Format, duration float64, report func(Progress)) error {
	if report == nil {
		report = func(Progress) {}
	}
	args := append([]string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-i", src}, convertArgs(f)...)
	args = append(args, "-progress", "pipe:1", "-nostats", dst)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start ffmpeg: %v", err)
	}
	report(Progress{Stage: "converting", Percent: -1})
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "out_time_us="); ok && duration > 0 {
			if us, err := strconv.ParseFloat(v, 64); err == nil {
				report(Progress{Stage: "converting", Percent: min(100, us/1e6/duration*100)})
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := lastLine(errBuf.String(), err.Error())
		if f.Audio && strings.Contains(errBuf.String(), "does not contain any stream") {
			msg = "this clip has no sound to extract"
		}
		return errors.New("ffmpeg: " + msg)
	}
	return nil
}

// ListDownloads returns the media files in dir, newest first.
func ListDownloads(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type f struct {
		name string
		mod  time.Time
	}
	var fs []f
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if in, err := e.Info(); err == nil {
			fs = append(fs, f{e.Name(), in.ModTime()})
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].mod.After(fs[j].mod) })
	out := make([]string, len(fs))
	for i, x := range fs {
		out[i] = filepath.Join(dir, x.name)
	}
	return out
}
