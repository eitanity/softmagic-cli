// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build linux || android || openbsd || dragonfly || solaris || aix

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
	// The conversions are needed where the fields are 32 bits (32-bit Linux).
	return time.Unix(int64(st.Atim.Sec), int64(st.Atim.Nsec)), true
}
