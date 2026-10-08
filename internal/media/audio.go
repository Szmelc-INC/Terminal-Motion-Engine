package media

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	sampleRate = 48000
	byteRate   = sampleRate * 2 * 2 // stereo s16
	chunkBytes = byteRate / 50      // 20 ms
	// audioLead is how far ahead of the clock audio is handed to the sink.
	audioLead = 0.06
)

// Clock reports the current media position in seconds and whether playback
// is running. Audio is paced against it, so pausing the clock pauses sound.
type Clock func() (pos float64, running bool)

// Audio plays a file's sound track. ffmpeg decodes it to PCM; the samples
// are metered out against the player clock into a system audio sink
// (pacat, pw-cat, aplay or ffplay — whichever is present). Keeping the pipe
// to the sink shallow is what keeps sound and picture in step.
type Audio struct {
	clock  Clock
	sink   *exec.Cmd
	sinkIn io.WriteCloser
	name   string

	mu      sync.Mutex
	dec     *exec.Cmd
	gen     atomic.Int64
	volume  atomic.Uint64 // float64 bits
	delay   atomic.Int64  // microseconds
	failed  atomic.Bool
	closing atomic.Bool
}

type sinkSpec struct {
	bin  string
	args []string
}

var sinks = []sinkSpec{
	{"pacat", []string{"--playback", "--raw", "--format=s16le", "--rate=48000", "--channels=2",
		"--latency-msec=40", "--client-name=termo", "--stream-name=termo"}},
	{"pw-cat", []string{"--playback", "--raw", "--format=s16", "--rate=48000", "--channels=2", "--latency=40ms", "-"}},
	{"aplay", []string{"-q", "-t", "raw", "-f", "S16_LE", "-r", "48000", "-c", "2", "-B", "60000", "-"}},
	{"ffplay", []string{"-nodisp", "-loglevel", "quiet", "-autoexit", "-fflags", "nobuffer", "-probesize", "32",
		"-f", "s16le", "-ar", "48000", "-ch_layout", "stereo", "-i", "pipe:0"}},
}

// NewAudio starts an audio sink. custom, when set, is a shell command that
// reads raw s16le/48 kHz/stereo PCM on stdin and replaces the built-in sinks.
func NewAudio(clock Clock, custom string) (*Audio, error) {
	a := &Audio{clock: clock}
	a.SetVolume(1)
	try := func(name string, cmd *exec.Cmd) bool {
		pr, pw, err := os.Pipe()
		if err != nil {
			return false
		}
		shrinkPipe(pw)
		cmd.Stdin = pr
		if err := cmd.Start(); err != nil {
			pr.Close()
			pw.Close()
			return false
		}
		pr.Close()
		exited := make(chan struct{})
		go func() { cmd.Wait(); close(exited) }()
		select {
		case <-exited: // died straight away: no server, bad flags…
			pw.Close()
			return false
		case <-time.After(120 * time.Millisecond):
		}
		go func() {
			<-exited
			if !a.closing.Load() {
				a.failed.Store(true)
			}
		}()
		a.sink, a.sinkIn, a.name = cmd, pw, name
		return true
	}
	if custom != "" {
		if try("custom", exec.Command("sh", "-c", custom)) {
			return a, nil
		}
		return nil, fmt.Errorf("audio sink command failed: %s", custom)
	}
	for _, s := range sinks {
		if _, err := exec.LookPath(s.bin); err != nil {
			continue
		}
		if try(s.bin, exec.Command(s.bin, s.args...)) {
			return a, nil
		}
	}
	return nil, errors.New("no working audio output found (tried pacat, pw-cat, aplay, ffplay)")
}

// Name returns the sink in use.
func (a *Audio) Name() string { return a.name }

// Failed reports whether the sink has died.
func (a *Audio) Failed() bool { return a.failed.Load() }

// SetVolume sets the gain (1 = unchanged, 0 = silent).
func (a *Audio) SetVolume(v float64) { a.volume.Store(math.Float64bits(v)) }

// SetDelay shifts sound later (positive) or earlier (negative), in seconds.
func (a *Audio) SetDelay(sec float64) { a.delay.Store(int64(sec * 1e6)) }

func atempo(speed float64) string {
	// Older ffmpeg limits each atempo instance to 0.5..2.
	var parts []string
	for speed < 0.5 {
		parts = append(parts, "atempo=0.5")
		speed /= 0.5
	}
	for speed > 2 {
		parts = append(parts, "atempo=2.0")
		speed /= 2
	}
	parts = append(parts, "atempo="+ftoa(speed))
	return strings.Join(parts, ",")
}

// Play (re)starts audio decoding at the given media position. Any previous
// stream is dropped.
func (a *Audio) Play(path string, start, speed float64, loop bool) {
	a.Stop()
	if a.failed.Load() {
		return
	}
	gen := a.gen.Load()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if loop {
		args = append(args, "-stream_loop", "-1")
	}
	if start > 0 {
		args = append(args, "-ss", ftoa(start))
	}
	args = append(args, netArgs(path)...)
	args = append(args, "-i", path, "-map", "0:a:0", "-vn", "-sn", "-dn")
	if math.Abs(speed-1) > 0.001 {
		args = append(args, "-af", atempo(speed))
	}
	args = append(args, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	a.mu.Lock()
	a.dec = cmd
	a.mu.Unlock()
	go a.feed(cmd, out, gen, start, speed)
}

func (a *Audio) feed(cmd *exec.Cmd, r io.Reader, gen int64, start, speed float64) {
	defer cmd.Wait()
	buf := make([]byte, chunkBytes)
	var read int64
	for a.gen.Load() == gen {
		if _, err := io.ReadFull(r, buf); err != nil {
			return
		}
		// Media time this chunk belongs to.
		mt := start + float64(read)/byteRate*speed
		read += int64(len(buf))
		for {
			if a.gen.Load() != gen {
				return
			}
			pos, running := a.clock()
			if !running {
				time.Sleep(4 * time.Millisecond)
				continue
			}
			ahead := (mt + float64(a.delay.Load())/1e6 - pos) / speed
			if ahead > audioLead {
				time.Sleep(time.Duration(math.Min(ahead-audioLead, 0.02) * float64(time.Second)))
				continue
			}
			if ahead < -0.25 {
				buf = buf[:0] // hopelessly late: skip to catch up
			}
			break
		}
		if len(buf) > 0 {
			if v := math.Float64frombits(a.volume.Load()); v != 1 {
				g := int32(v * 256)
				for i := 0; i+1 < len(buf); i += 2 {
					s := int32(int16(binary.LittleEndian.Uint16(buf[i:]))) * g >> 8
					if s > 32767 {
						s = 32767
					} else if s < -32768 {
						s = -32768
					}
					binary.LittleEndian.PutUint16(buf[i:], uint16(int16(s)))
				}
			}
			if _, err := a.sinkIn.Write(buf); err != nil {
				a.failed.Store(true)
				return
			}
		}
		buf = buf[:chunkBytes]
	}
}

// Stop drops the current stream; the sink stays open.
func (a *Audio) Stop() {
	a.gen.Add(1)
	a.mu.Lock()
	if a.dec != nil && a.dec.Process != nil {
		a.dec.Process.Kill()
	}
	a.dec = nil
	a.mu.Unlock()
}

// Close stops playback and shuts the sink down.
func (a *Audio) Close() {
	a.closing.Store(true)
	a.Stop()
	a.sinkIn.Close()
	if a.sink.Process != nil {
		a.sink.Process.Kill()
	}
}
