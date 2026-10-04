// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build linux

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
)

// TestNoDescriptorLeaks runs the command over every kind of input and every
// path that opens something, many times, and requires the process to hold
// the same descriptors afterwards. No linter finds an *os.File that is never
// closed; on Unix the leak is silent until descriptors run out, and on
// Windows it shows up as a file that cannot be deleted. /proc/self/fd makes
// it countable here.
func TestNoDescriptorLeaks(t *testing.T) {
	d := t.TempDir()
	text := filepath.Join(d, "note.txt")
	writeFile(t, text, "hello\n")
	empty := filepath.Join(d, "empty")
	writeFile(t, empty, "")
	dir := filepath.Join(d, "dir")
	fifo := filepath.Join(d, "fifo")
	link := filepath.Join(d, "link")
	dangling := filepath.Join(d, "dangling")
	list := filepath.Join(d, "list")
	magic := filepath.Join(d, "magic")
	for _, err := range []error{
		os.Mkdir(dir, 0o700),
		mkfifo(fifo),
		os.Symlink(text, link),
		os.Symlink("nowhere", dangling),
		os.WriteFile(list, []byte(text+"\n"+empty+"\n"+dir+"\n"), 0o600),
		os.Mkdir(magic, 0o700),
		os.WriteFile(filepath.Join(magic, "leak"), []byte("0\tstring\thello\tleak test\n"), 0o600),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(d, "missing")
	inputs := []string{text, empty, dir, fifo, link, dangling, missing, "/dev/null"}
	if _, err := os.Stat("/dev/ptmx"); err == nil {
		// A device that answers a non-blocking read with EAGAIN: the error path.
		inputs = append(inputs, "/dev/ptmx")
	}

	var runs [][]string
	for _, mode := range [][]string{{}, {"-L"}, {"-i"}, {"--extension"}, {"--apple"}, {"-s"}, {"-E"}, {"--json"}} {
		runs = append(runs, append(append([]string{}, mode...), inputs...))
	}
	runs = append(runs,
		[]string{"-"},               // standard input
		[]string{"-f", list},        // a list file
		[]string{"-f", "-"},         // a list on standard input
		[]string{"-m", magic, text}, // a rule directory instead of the embedded database
		[]string{"--extra-magic", magic, text},
		[]string{"-f", missing},       // a list file that cannot be opened
		[]string{"-m", missing, text}, // a rule directory that cannot be read
	)
	all := func() {
		for _, args := range runs {
			capture(t, text+"\n", args...)
		}
	}

	all() // the runtime opens its poller's descriptors on first use, once
	// An unreachable *os.File is closed by its finalizer after a collection,
	// which hides a leak on Unix (and comes too late on Windows, where the
	// directory is deleted first). With the collector off, a file the code
	// forgot to close is still open when the count is taken.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	runtime.GC()
	before := openDescriptors(t)
	for range 20 {
		all()
	}
	after := openDescriptors(t)

	var leaked []string
	for fd, target := range after {
		if before[fd] != target {
			leaked = append(leaked, fd+" -> "+target)
		}
	}
	if len(leaked) > 0 {
		sort.Strings(leaked)
		t.Fatalf("%d descriptors before, %d after; new or changed:\n  %s",
			len(before), len(after), strings.Join(leaked, "\n  "))
	}
}

// openDescriptors maps each descriptor the process holds to what it refers
// to. The listing's own descriptor is in both snapshots under the same
// number, so it cancels out.
func openDescriptors(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot list descriptors: %v", err)
	}
	fds := make(map[string]string, len(entries))
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			continue // the listing's own descriptor, closed by the time it is read
		}
		fds[e.Name()] = target
	}
	return fds
}
