// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestSpecialFiles covers the Unix stat layer against answers transcribed
// from the reference: symlinks, a fifo, setgid, a name with a tab, and the
// errors as Unix spells them. None of these can be made, or reads the same,
// on Windows, whose stat layer has not been compared with a reference.
func TestSpecialFiles(t *testing.T) {
	d := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(d, "dir"), 0o700))
	must(os.WriteFile(filepath.Join(d, "empty"), nil, 0o600))
	must(os.WriteFile(filepath.Join(d, "note.txt"), []byte("hello\n"), 0o600))
	must(os.Symlink("note.txt", filepath.Join(d, "link")))
	must(os.Symlink("nowhere", filepath.Join(d, "dangling")))
	must(mkfifo(filepath.Join(d, "fifo")))
	must(os.WriteFile(filepath.Join(d, "sg.txt"), []byte("xy"), 0o600))
	must(os.Chmod(filepath.Join(d, "sg.txt"), 0o700|os.ModeSetgid))
	must(os.WriteFile(filepath.Join(d, "tab\tname"), []byte("x"), 0o600))
	must(os.Symlink(filepath.Join(d, "tab\tname"), filepath.Join(d, "tablink")))
	cases := []struct {
		flags []string
		file  string
		want  string
		code  int
	}{
		{nil, "dir", "directory", 0},
		{nil, "empty", "empty", 0},
		{nil, "link", "symbolic link to note.txt", 0},
		{nil, "dangling", "broken symbolic link to nowhere", 0},
		{nil, "fifo", "fifo (named pipe)", 0},
		{nil, "missing", "cannot open `" + filepath.Join(d, "missing") + "' (No such file or directory)", 0},
		{nil, "sg.txt", "setgid , ASCII text, with no line terminators", 0},
		{nil, "tablink", "symbolic link to " + strings.ReplaceAll(filepath.Join(d, "tab\\011name"), "\\011", "\\011"), 0},
		{[]string{"-r"}, "tablink", "symbolic link to " + filepath.Join(d, "tab\tname"), 0},
		{[]string{"-L"}, "link", "ASCII text", 0},
		{[]string{"-L"}, "dangling", "cannot open `" + filepath.Join(d, "dangling") + "' (No such file or directory)", 0},
		{[]string{"-i"}, "dir", "inode/directory; charset=binary", 0},
		{[]string{"-i"}, "empty", "inode/x-empty; charset=binary", 0},
		{[]string{"-i"}, "link", "inode/symlink; charset=binary", 0},
		{[]string{"-i"}, "dangling", "inode/symlink", 0},
		{[]string{"-i"}, "fifo", "inode/fifo; charset=binary", 0},
		{[]string{"-i"}, "sg.txt", "text/plain; charset=us-ascii", 0},
		{[]string{"--mime-type"}, "dangling", "inode/symlink", 0},
		{[]string{"--mime-encoding"}, "dir", "binary", 0},
		{[]string{"--mime-encoding"}, "dangling", "ERROR: (null)", 1},
		{[]string{"--extension"}, "dir", "ERROR: cannot read `" + filepath.Join(d, "dir") + "' (Is a directory)", 1},
		{[]string{"--extension"}, "empty", "???", 0},
		{[]string{"--extension"}, "dangling", "broken symbolic link to nowhere", 0},
		{[]string{"--extension"}, "fifo", "writable, no read permission", 0},
		{[]string{"--apple"}, "dir", "ERROR: cannot read `" + filepath.Join(d, "dir") + "' (Is a directory)", 1},
		{[]string{"--apple"}, "note.txt", "UNKNUNKN", 0},
		{[]string{"-s"}, "fifo", "ERROR: (null)", 1},
		{[]string{"-s"}, "empty", "empty", 0},
		{[]string{"-s", "-i"}, "empty", "application/x-empty; charset=binary", 0},
		{[]string{"-E"}, "missing", "ERROR: cannot stat `" + filepath.Join(d, "missing") + "' (No such file or directory)", 1},
		{[]string{"-E"}, "dangling", "ERROR: broken symbolic link to nowhere (No such file or directory)", 1},
		{[]string{"-E"}, "dir", "directory", 0},
	}
	for _, c := range cases {
		args := append(append([]string{"-b"}, c.flags...), filepath.Join(d, c.file))
		out, _, code := capture(t, "", args...)
		if got := strings.TrimSuffix(out, "\n"); got != c.want || code != c.code {
			t.Errorf("%v %s: got %q exit %d, want %q exit %d", c.flags, c.file, got, code, c.want, c.code)
		}
	}
}

func mkfifo(name string) error { return syscall.Mkfifo(name, 0o600) }
