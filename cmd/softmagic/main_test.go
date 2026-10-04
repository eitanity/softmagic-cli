// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("TZ", "UTC"); err != nil { // the corpus expectations were made under TZ=UTC
		panic(err)
	}
	os.Exit(m.Run())
}

// capture runs the command with its output in files, as run takes
// *os.File, and returns stdout, stderr and the exit code.
func capture(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	in, err := os.CreateTemp(dir, "in")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.WriteString(stdin); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(dir, "out")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(dir, "err")
	if err != nil {
		t.Fatal(err)
	}
	code := run(args, in, out, errf)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errf.Name())
	return string(o), string(e), code
}

// writeFile creates a private test file.
func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// corpusDir is the library's parity corpus: each .testfile beside a
// .expect of four lines (-b, -b -i, -b --extension, -b --apple) made by
// the reference.
func corpusDir(t *testing.T) string {
	t.Helper()
	// The test runs in cmd/softmagic; the library checkout sits beside the
	// repository root.
	dir := filepath.Join("..", "..", "..", "softmagic", "testdata", "corpus")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("library corpus not beside this checkout")
	}
	return dir
}

func TestCorpus(t *testing.T) {
	dir := corpusDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.testfile"))
	if err != nil || len(files) == 0 {
		t.Fatal("no corpus files")
	}
	flags := [][]string{{"-b"}, {"-b", "-i"}, {"-b", "--extension"}, {"-b", "--apple"}}
	for _, f := range files {
		want, err := os.ReadFile(strings.TrimSuffix(f, ".testfile") + ".expect")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
		if len(lines) != 4 {
			t.Fatalf("%s: expect file has %d lines", f, len(lines))
		}
		for i, fl := range flags {
			out, errOut, code := capture(t, "", append(fl, f)...)
			if code != 0 || errOut != "" {
				t.Errorf("%s %v: exit %d, stderr %q", f, fl, code, errOut)
			}
			if got := strings.TrimSuffix(out, "\n"); got != lines[i] {
				t.Errorf("%s %v:\n got %q\nwant %q", f, fl, got, lines[i])
			}
		}
	}
}

func TestPadding(t *testing.T) {
	dir := corpusDir(t)
	a, b := filepath.Join(dir, "a.testfile"), filepath.Join(dir, "bzip3.testfile")
	out, _, _ := capture(t, "", "--mime-type", a, b)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], a+":     ") || !strings.HasPrefix(lines[1], b+": ") {
		t.Fatalf("padding wrong:\n%s", out)
	}
	out, _, _ = capture(t, "", "-N", "--mime-type", a, b)
	if !strings.HasPrefix(out, a+": ") {
		t.Fatalf("-N wrong:\n%s", out)
	}
	out, _, _ = capture(t, "", "-F", " => ", "--mime-type", a)
	if !strings.HasPrefix(out, a+" =>  ") {
		t.Fatalf("-F wrong:\n%s", out)
	}
	out, _, _ = capture(t, "", "-0", "--mime-type", a)
	if !strings.HasPrefix(out, a+"\x00: ") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("-0 wrong: %q", out)
	}
	out, _, _ = capture(t, "", "-00", "--mime-type", a)
	if !strings.HasPrefix(out, a+"\x00") || strings.Contains(out, ":") || !strings.HasSuffix(out, "\x00") {
		t.Fatalf("-00 wrong: %q", out)
	}
}

// TestSpecialFiles checks the stat layer against the reference's
// observed output for every mode.
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

func TestStdinAndLists(t *testing.T) {
	out, _, code := capture(t, "#!/bin/sh\necho hi\n", "-")
	if code != 0 || out != "/dev/stdin: POSIX shell script, ASCII text executable\n" {
		t.Fatalf("stdin: %q exit %d", out, code)
	}
	out, _, _ = capture(t, "", "-b", "-")
	if out != "empty\n" {
		t.Fatalf("empty stdin: %q", out)
	}
	d := t.TempDir()
	short, long := filepath.Join(d, "a"), filepath.Join(d, "longer")
	writeFile(t, short, "hello")
	writeFile(t, long, "world")
	out, _, code = capture(t, short+"\n"+long+"\n", "-f", "-")
	want := short + ":      ASCII text, with no line terminators\n" + long + ": ASCII text, with no line terminators\n"
	if code != 0 || out != want {
		t.Fatalf("-f -: %q exit %d", out, code)
	}
	out, _, _ = capture(t, short+"\n"+long+"\n", "-n", "-f", "-")
	want = short + ": ASCII text, with no line terminators\n" + long + ": ASCII text, with no line terminators\n"
	if out != want {
		t.Fatalf("-n -f -: %q", out)
	}
}

func TestOptions(t *testing.T) {
	out, errOut, code := capture(t, "")
	if code != 1 || out != "" || !strings.HasPrefix(errOut, "Usage:") {
		t.Fatalf("no files: %q %q %d", out, errOut, code)
	}
	for _, bad := range [][]string{{"-k", "x"}, {"-z", "x"}, {"-P", "elf_phnum=1", "x"}, {"-Q", "x"}} {
		if _, errOut, code := capture(t, "", bad...); code != 1 || errOut == "" {
			t.Errorf("%v accepted", bad)
		}
	}
	out, _, code = capture(t, "", "--version")
	if code != 0 || !strings.Contains(out, "implements file 5.48") {
		t.Fatalf("--version: %q", out)
	}
	d := t.TempDir()
	f := filepath.Join(d, "sh")
	writeFile(t, f, "#!/bin/sh\necho\n")
	out, _, _ = capture(t, "", "--json", f)
	if !strings.Contains(out, `"name":"`+f+`"`) || !strings.Contains(out, `"Phase":"text"`) {
		t.Fatalf("--json: %q", out)
	}
	f = filepath.Join(d, "txt")
	writeFile(t, f, "hello world\n")
	out, _, _ = capture(t, "", "-b", "-P", "bytes=5", f)
	if out != "ASCII text, with no line terminators\n" {
		t.Fatalf("-P bytes=5: %q", out)
	}
	out, _, _ = capture(t, "", "-b", "--max-bytes", "5", f)
	if out != "ASCII text, with no line terminators\n" {
		t.Fatalf("--max-bytes 5: %q", out)
	}
}

func TestMagicDirs(t *testing.T) {
	dir := filepath.Join("..", "softmagic", "magic", "Magdir")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("library Magdir not beside this checkout")
	}
	c := corpusDir(t)
	f := filepath.Join(c, "a.testfile")
	want, _, _ := capture(t, "", "-b", f)
	got, errOut, code := capture(t, "", "-m", dir, "-b", f)
	if code != 0 || got != want {
		t.Fatalf("-m: %q %q exit %d", got, errOut, code)
	}
	extra := t.TempDir()
	writeFile(t, filepath.Join(extra, "mine"), "0\tstring\tMYMAGIC\tmy own format\n!:mime\tapplication/x-mine\n")
	mine := filepath.Join(t.TempDir(), "sample") // not in extra: every file there is magic
	writeFile(t, mine, "MYMAGIC here\n")
	got, errOut, code = capture(t, "", "--extra-magic", extra, "-i", "-b", mine)
	if code != 0 || got != "application/x-mine; charset=us-ascii\n" {
		t.Fatalf("--extra-magic: %q %q exit %d", got, errOut, code)
	}
	if _, _, code = capture(t, "", "-m", filepath.Join(extra, "none"), mine); code != 1 {
		t.Fatal("missing -m directory accepted")
	}
}

func TestEscapeOutput(t *testing.T) {
	cases := map[string]string{
		"plain":         "plain",
		"tab\there":     "tab\\011here",
		"ü ok":          "ü ok",
		"\xff raw":      "\\377 raw",
		"a\u200bb":      "a\\342\\200\\213b",
		"\x01 and \xff": "\\001 and \\377",
	}
	for in, want := range cases {
		if got := escapeOutput(in); got != want {
			t.Errorf("escapeOutput(%q) = %q, want %q", in, got, want)
		}
	}
	if nameText("tab\tx", false) != "tab\\011x" || nameWidth("tab\tx", false) != 8 || nameWidth("ü", false) != 1 {
		t.Error("name escaping")
	}
	var buf bytes.Buffer
	buf.WriteString(nameText("\xff", false))
	if buf.String() != "\\377" {
		t.Error("invalid byte in name")
	}
}
