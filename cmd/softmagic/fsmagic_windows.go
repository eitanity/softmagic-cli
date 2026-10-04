// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build windows

package main

import (
	"io/fs"
	"os"
)

// deviceNumbers has no Windows equivalent; the reference prints none there.
func deviceNumbers(os.FileInfo) string { return "" }

// unreadableInfo is unreadable_info: a file that could be stat'ed but
// not opened or read. Windows has no access(2); the reference consults
// the permission bits, as does this.
func unreadableInfo(name string, m fs.FileMode) string {
	s := ""
	if m.Perm()&0o200 != 0 {
		s += "writable, "
	}
	if m.IsRegular() {
		s += "regular file, "
	}
	return s + "no read permission"
}

// openForRead opens a file read-only; Windows has no non-blocking flag.
func openForRead(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY, 0)
}
