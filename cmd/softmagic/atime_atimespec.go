// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build darwin || ios || freebsd || netbsd

package main

import (
	"os"
	"syscall"
	"time"
)

// accessTime is the file's last access time, for -p.
func accessTime(info os.FileInfo) (time.Time, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(st.Atimespec.Sec), int64(st.Atimespec.Nsec)), true
}
