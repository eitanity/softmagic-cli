// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build unix

package main

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// deviceNumbers is " (major/minor)" from the raw stat, as glibc splits
// them.
func deviceNumbers(info os.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	rdev := uint64(st.Rdev)
	major := ((rdev >> 8) & 0xfff) | ((rdev >> 32) & 0xfffff000)
	minor := (rdev & 0xff) | ((rdev >> 12) & 0xffffff00)
	return fmt.Sprintf(" (%d/%d)", major, minor)
}

// unreadableInfo is unreadable_info: a file that could be stat'ed but
// not opened or read.
func unreadableInfo(name string, m fs.FileMode) string {
	s := ""
	if syscall.Access(name, 2) == nil {
		s += "writable, "
	}
	if syscall.Access(name, 1) == nil {
		s += "executable, "
	}
	if m.IsRegular() {
		s += "regular file, "
	}
	return s + "no read permission"
}

// openForRead opens a file as file(1) does: read-only and non-blocking,
// so a fifo with no writer reads as empty instead of waiting.
func openForRead(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
