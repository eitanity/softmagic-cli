// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build !(linux || android || openbsd || dragonfly || solaris || aix || darwin || ios || freebsd || netbsd || windows)

package main

import (
	"os"
	"time"
)

// accessTime is not known on this platform, so -p restores nothing.
func accessTime(os.FileInfo) (time.Time, bool) { return time.Time{}, false }
