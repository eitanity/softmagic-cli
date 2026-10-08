// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bytes"
	"encoding/json"
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
//
// Each file is closed when capture returns, on every path: t.Fatal runs
// deferred calls. Windows refuses to delete an open file, so a file left
// open fails t.TempDir's cleanup there, and a caller that runs thousands
// of captures in one test would otherwise run out of descriptors.
func capture(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	create := func(name string) *os.File {
		t.Helper()
		f, err := os.CreateTemp(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	closeFile := func(f *os.File) {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}
	in := create("in") // inputFile
	defer closeFile(in)
	out := create("out")
	defer closeFile(out)
	errf := create("err")
	defer closeFile(errf)
	if _, err := in.WriteString(stdin); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Seek(0, 0); err != nil {
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
	for _, f := range files { // fileName
		want, err := os.ReadFile(strings.TrimSuffix(f, ".testfile") + ".expect")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
		if len(lines) != 4 {
			t.Fatalf("%s: expect file has %d lines", f, len(lines))
		}
		for i, fl := range flags { // flagSet
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
	a, b := filepath.Join(dir, "a.testfile"), filepath.Join(dir, "bzip3.testfile") // bzip3File
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
	for _, bad := range [][]string{{"-z", "x"}, {"-P", "nosuch=1", "x"}, {"-Q", "x"}} {
		if _, errOut, code := capture(t, "", bad...); code != 1 || errOut == "" {
			t.Errorf("%v accepted", bad)
		}
	}
	out, _, code = capture(t, "", "--version")
	if code != 0 || !strings.HasPrefix(out, "softmagic-cli ") || !strings.Contains(out, "\nlibrary ") ||
		!strings.Contains(out, "implements file 5.48") {
		t.Fatalf("--version: %q", out)
	}
	d := t.TempDir()            // tempDir
	f := filepath.Join(d, "sh") // scriptPath
	writeFile(t, f, "#!/bin/sh\necho\n")
	out, _, _ = capture(t, "", "--json", f)
	// The name is compared as JSON encodes it: a Windows path's backslashes
	// are escaped.
	name, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name":`+string(name)) || !strings.Contains(out, `"Phase":"text"`) {
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
