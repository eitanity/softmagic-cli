// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

// accessTime is the file's last access time, for -p.
func accessTime(info os.FileInfo) (time.Time, bool) {
	d, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, d.LastAccessTime.Nanoseconds()), true
}
