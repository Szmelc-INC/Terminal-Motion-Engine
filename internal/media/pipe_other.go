//go:build !linux

package media

import "os"

func shrinkPipe(*os.File) {}
