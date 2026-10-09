package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
)

// IsURL reports whether a path is a web address rather than a file.
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// netArgs are the ffmpeg input options that keep a web stream going
// through short stalls. HLS playlists do their own reconnecting.
func netArgs(path string) []string {
	if !IsURL(path) || strings.Contains(path, ".m3u8") {
		return nil
	}
	return []string{"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "4"}
}

// Picture is a decoded RGB24 still.
type Picture struct {
	Pix  []byte
	W, H int
}

// Thumb decodes the first frame of a picture, GIF or video (a file or a
// URL), scaled down to at most maxW pixels wide.
func Thumb(ctx context.Context, src string, maxW int) (*Picture, error) {
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	args = append(args, netArgs(src)...)
	args = append(args, "-i", src, "-frames:v", "1", "-an",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2:flags=area", maxW), "-f", "image2pipe", "-c:v", "png", "pipe:1")
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	p := &Picture{W: b.Dx(), H: b.Dy(), Pix: make([]byte, b.Dx()*b.Dy()*3)}
	if p.W == 0 || p.H == 0 {
		return nil, errors.New("empty picture")
	}
	rgba, ok := img.(*image.RGBA)
	nrgba, ok2 := img.(*image.NRGBA)
	for y := 0; y < p.H; y++ {
		for x := 0; x < p.W; x++ {
			o := (y*p.W + x) * 3
			switch {
			case ok:
				i := rgba.PixOffset(b.Min.X+x, b.Min.Y+y)
				copy(p.Pix[o:o+3], rgba.Pix[i:i+3])
			case ok2:
				i := nrgba.PixOffset(b.Min.X+x, b.Min.Y+y)
				copy(p.Pix[o:o+3], nrgba.Pix[i:i+3])
			default:
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				p.Pix[o], p.Pix[o+1], p.Pix[o+2] = uint8(r>>8), uint8(g>>8), uint8(bl>>8)
			}
		}
	}
	return p, nil
}

// Resize returns the picture scaled to exactly w×h by averaging the source
// pixels each target pixel covers.
func (p *Picture) Resize(w, h int) []byte {
	out := make([]byte, w*h*3)
	if p.W == 0 || p.H == 0 || w < 1 || h < 1 {
		return out
	}
	for y := 0; y < h; y++ {
		y0, y1 := y*p.H/h, max((y+1)*p.H/h, y*p.H/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*p.W/w, max((x+1)*p.W/w, x*p.W/w+1)
			var r, g, b, n int
			for sy := y0; sy < y1 && sy < p.H; sy++ {
				row := p.Pix[(sy*p.W+x0)*3:]
				for sx := x0; sx < x1 && sx < p.W; sx++ {
					r += int(row[0])
					g += int(row[1])
					b += int(row[2])
					row = row[3:]
					n++
				}
			}
			if n > 0 {
				o := (y*w + x) * 3
				out[o], out[o+1], out[o+2] = uint8(r/n), uint8(g/n), uint8(b/n)
			}
		}
	}
	return out
}
