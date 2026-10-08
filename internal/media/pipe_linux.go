//go:build linux

package media

import (
	"os"

	"golang.org/x/sys/unix"
)

// shrinkPipe makes the pipe to the audio sink as small as the kernel allows
// (one page). The default 64 KiB would hold a third of a second of audio,
// which is a third of a second the picture could run ahead of the sound.
func shrinkPipe(f *os.File) {
	unix.FcntlInt(f.Fd(), unix.F_SETPIPE_SZ, 4096)
}
