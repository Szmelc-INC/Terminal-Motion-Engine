package media

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Frame is one decoded RGB24 picture.
type Frame struct {
	Pix  []byte
	W, H int
	PTS  float64 // seconds on the (unwrapped) media timeline
}

// VideoOpts configures a decoder run. A run is immutable: seeking or
// resizing means starting a new one.
type VideoOpts struct {
	Path         string
	PreInput     []string // ffmpeg options placed before the input
	Start        float64  // seek position in the file, seconds
	Base         float64  // timeline position of the first frame
	W, H         int      // output size in pixels
	FPS          float64  // output frame rate
	CropX, CropY float64  // fraction of the source to keep (1 = all)
	Loop         bool     // loop the input seamlessly
	Still        bool     // single picture
	HWAccel      bool
}

// Video is a running ffmpeg decode. Frames arrive on Frames, already scaled
// to the requested size, and the channel is closed at end of stream.
type Video struct {
	VideoOpts
	Frames chan *Frame

	cmd    *exec.Cmd
	free   chan *Frame
	done   chan struct{}
	once   sync.Once
	stderr bytes.Buffer
	errMu  sync.Mutex
	err    error
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 6, 64) }

// StartVideo launches ffmpeg and begins decoding.
func StartVideo(o VideoOpts) (*Video, error) {
	if o.W < 1 || o.H < 1 {
		return nil, fmt.Errorf("invalid video size %dx%d", o.W, o.H)
	}
	if o.FPS <= 0 {
		o.FPS = 25
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if o.HWAccel {
		args = append(args, "-hwaccel", "auto")
	}
	if o.Loop && !o.Still {
		args = append(args, "-stream_loop", "-1")
	}
	if o.Start > 0 && !o.Still {
		args = append(args, "-ss", ftoa(o.Start))
	}
	args = append(args, o.PreInput...)
	args = append(args, "-i", o.Path, "-map", "0:v:0", "-an", "-sn", "-dn")
	var vf []string
	if !o.Still {
		vf = append(vf, "fps="+ftoa(o.FPS))
	}
	if (o.CropX > 0 && o.CropX < 0.999) || (o.CropY > 0 && o.CropY < 0.999) {
		cx, cy := o.CropX, o.CropY
		if cx <= 0 || cx > 1 {
			cx = 1
		}
		if cy <= 0 || cy > 1 {
			cy = 1
		}
		vf = append(vf, fmt.Sprintf("crop=iw*%s:ih*%s", ftoa(cx), ftoa(cy)))
	}
	vf = append(vf, fmt.Sprintf("scale=%d:%d:flags=area", o.W, o.H), "setsar=1")
	args = append(args, "-vf", strings.Join(vf, ","))
	if o.Still {
		args = append(args, "-frames:v", "1")
	}
	args = append(args, "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1")

	v := &Video{
		VideoOpts: o,
		Frames:    make(chan *Frame, 3),
		free:      make(chan *Frame, 8),
		done:      make(chan struct{}),
	}
	v.cmd = exec.Command("ffmpeg", args...)
	v.cmd.Stderr = &v.stderr
	out, err := v.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := v.cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start ffmpeg: %v", err)
	}
	go v.read(out)
	return v, nil
}

func (v *Video) read(r io.Reader) {
	defer close(v.Frames)
	size := v.W * v.H * 3
	produced := 0
	for {
		var f *Frame
		select {
		case f = <-v.free:
		default:
			f = &Frame{Pix: make([]byte, size), W: v.W, H: v.H}
		}
		if _, err := io.ReadFull(r, f.Pix); err != nil {
			break
		}
		f.PTS = v.Base + float64(produced)/v.FPS
		produced++
		select {
		case v.Frames <- f:
		case <-v.done:
			v.cmd.Wait()
			return
		}
	}
	werr := v.cmd.Wait()
	select {
	case <-v.done:
		return
	default:
	}
	if produced == 0 {
		msg := strings.TrimSpace(v.stderr.String())
		if i := strings.IndexByte(msg, '\n'); i > 0 {
			msg = msg[:i]
		}
		if msg == "" && werr != nil {
			msg = werr.Error()
		}
		if msg == "" {
			msg = "no frames decoded"
		}
		v.errMu.Lock()
		v.err = fmt.Errorf("ffmpeg: %s", msg)
		v.errMu.Unlock()
	}
}

// Err reports why decoding produced nothing, if it failed.
func (v *Video) Err() error {
	v.errMu.Lock()
	defer v.errMu.Unlock()
	return v.err
}

// Recycle hands a frame's buffer back to the decoder for reuse.
func (v *Video) Recycle(f *Frame) {
	if f == nil || f.W != v.W || f.H != v.H {
		return
	}
	select {
	case v.free <- f:
	default:
	}
}

// Close stops the decoder.
func (v *Video) Close() {
	v.once.Do(func() {
		close(v.done)
		if v.cmd.Process != nil {
			v.cmd.Process.Kill()
		}
	})
}
