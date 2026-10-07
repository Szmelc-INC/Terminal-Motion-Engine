// Package media decodes video and audio by driving ffmpeg as a subprocess.
package media

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Info describes a media file.
type Info struct {
	Path     string
	Width    int     // display width in pixels (after rotation and SAR)
	Height   int     // display height in pixels
	FPS      float64 // source frame rate
	Duration float64 // seconds; 0 when unknown (e.g. a still image)
	Frames   int
	HasAudio bool
	Codec    string
	Still    bool
}

// Aspect returns the display aspect ratio (width / height).
func (i Info) Aspect() float64 {
	if i.Height == 0 {
		return 16.0 / 9
	}
	return float64(i.Width) / float64(i.Height)
}

// CheckTools verifies that ffmpeg and ffprobe are installed.
func CheckTools() error {
	for _, t := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(t); err != nil {
			return fmt.Errorf("%s not found in PATH — termo uses ffmpeg to decode media, please install it", t)
		}
	}
	return nil
}

func ratio(s string) float64 {
	a, b, ok := strings.Cut(s, "/")
	if !ok {
		a, b, ok = strings.Cut(s, ":")
	}
	n, _ := strconv.ParseFloat(a, 64)
	if !ok {
		return n
	}
	d, _ := strconv.ParseFloat(b, 64)
	if d == 0 {
		return 0
	}
	return n / d
}

// Probe reads stream information with ffprobe.
func Probe(path string) (Info, error) {
	info := Info{Path: path}
	if st, err := os.Stat(path); err != nil {
		return info, err
	} else if st.IsDir() {
		return info, fmt.Errorf("%s is a directory", path)
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return info, fmt.Errorf("cannot read %s: %s", path, msg)
	}
	var doc struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			AvgRate     string `json:"avg_frame_rate"`
			RRate       string `json:"r_frame_rate"`
			NbFrames    string `json:"nb_frames"`
			Duration    string `json:"duration"`
			SAR         string `json:"sample_aspect_ratio"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
			SideData []struct {
				Rotation float64 `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Name     string `json:"format_name"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return info, fmt.Errorf("cannot parse ffprobe output for %s: %v", path, err)
	}
	found := false
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "audio":
			info.HasAudio = true
		case "video":
			if found || s.Disposition.AttachedPic == 1 {
				continue
			}
			found = true
			w, h := float64(s.Width), float64(s.Height)
			if sar := ratio(s.SAR); sar > 0 {
				w *= sar
			}
			for _, sd := range s.SideData {
				if r := math.Mod(math.Abs(sd.Rotation), 180); r > 45 && r < 135 {
					w, h = h, w
				}
			}
			info.Width, info.Height = int(math.Round(w)), int(math.Round(h))
			info.Codec = s.CodecName
			info.FPS = ratio(s.AvgRate)
			if info.FPS <= 0 || info.FPS > 1000 {
				info.FPS = ratio(s.RRate)
			}
			info.Frames, _ = strconv.Atoi(s.NbFrames)
			info.Duration, _ = strconv.ParseFloat(s.Duration, 64)
		}
	}
	if !found {
		return info, errors.New(path + ": no video stream found")
	}
	if d, _ := strconv.ParseFloat(doc.Format.Duration, 64); d > info.Duration {
		info.Duration = d
	}
	if info.FPS <= 0 || info.FPS > 1000 {
		info.FPS = 25
	}
	// Single pictures come through image demuxers and report one frame.
	if strings.Contains(doc.Format.Name, "image2") || strings.HasSuffix(doc.Format.Name, "_pipe") || info.Frames == 1 {
		if info.Frames <= 1 {
			info.Still = true
			info.Duration = 0
		}
	}
	return info, nil
}
