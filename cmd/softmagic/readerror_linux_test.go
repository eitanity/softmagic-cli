// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

// eioFile is a sysfs file that stat calls a 4 KiB regular file and whose
// read fails with EIO on hosts without CPU power management.
const eioFile = "/sys/devices/system/cpu/power/autosuspend_delay_ms"

// TestReadErrorRegularFile: a regular file whose read fails is reported
// as the reference reports it, with exit status 1, not identified as
// data. It skips on hosts where the file reads.
func TestReadErrorRegularFile(t *testing.T) {
	info, err := os.Stat(eioFile)
	if err != nil || !info.Mode().IsRegular() {
		t.Skipf("%s is not a regular file here", eioFile)
	}
	if _, err := os.ReadFile(eioFile); !errors.Is(err, syscall.EIO) {
		t.Skipf("%s reads here without EIO (%v)", eioFile, err)
	}
	want := eioFile + ": ERROR: cannot read `" + eioFile + "' (Input/output error)"
	for _, flags := range [][]string{nil, {"-i"}, {"-k"}} {
		out, _, code := capture(t, "", append(flags, eioFile)...)
		if got := strings.TrimSuffix(out, "\n"); got != want || code != 1 {
			t.Errorf("%v: got %q exit %d, want %q exit 1", flags, got, code, want)
		}
	}
}
