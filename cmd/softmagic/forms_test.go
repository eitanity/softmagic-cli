// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOptionForms runs every way file(1)'s getopt_long takes an option
// through both programs: long names, abbreviations, clustered short
// options, attached values, options after names, "--", POSIXLY_CORRECT,
// -b given twice, -0 counted, and -f read where it appears. Standard output
// and the exit code must match; standard error is this program's own words
// (its name and usage text), so it is only required to be empty when the
// reference's is. It needs the reference, as TestReference does.
func TestOptionForms(t *testing.T) {
	ref := referenceDir(t)
	corpus := filepath.Join("..", "..", "..", "softmagic", "testdata", "corpus")
	a, b := filepath.Join(corpus, "json1.testfile"), filepath.Join(corpus, "pnm1.testfile") // pnmFile
	if _, err := os.Stat(a); err != nil {
		t.Skip("library corpus not beside this checkout")
	}
	list := filepath.Join(t.TempDir(), "list")
	writeFile(t, list, a+"\n")
	odd := filepath.Join(t.TempDir(), "odd")
	writeFile(t, odd, a+"\n\n"+b+"\r\nno-newline") // an empty name, a CR kept, a last line unended
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(b, link); err != nil {
		t.Skip("no symlinks here")
	}
	forms := [][]string{
		{"--brief", a}, {"--mime", a}, {"--keep-going", a}, {"--raw", a}, {"--no-pad", a, b},
		{"--no-buffer", a}, {"--special-files", a}, {"--preserve-date", a}, {"--no-sandbox", a},
		{"--exclude", "json", a}, {"--exclude=json", a}, {"--exclude-quiet", "nosuch", a},
		{"--separator", "@", a}, {"--separator=@", a}, {"--parameter", "bytes=10", a},
		{"--magic-file", ""}, {"--bri", a}, {"--mime-t", a}, {"--mime-e", a}, {"--ext", a}, {"--ap", a},
		{"--keep", "--mime-type", a}, {"-bi", a}, {"-bk", a}, {"-kib", a}, {"-Nb", a, b},
		{"-ejson", a}, {"-Pbytes=10", a}, {"-F@", a}, {"-b", "-b", a}, {"-bb", a}, {"-b", "-b", a, b},
		{"-bbb", a, b}, {a, "-b"}, {a, "-i", b}, {"-b", "--", a}, {"-0", a}, {"-00", a},
		{"--print0", "--print0", a}, {"-f", list}, {"--files-from", list, "-i", b},
		{"-f", list, "-b", b}, {"-i", "-f", list, b}, {"-f", list, "-e", "json", b},
		{"-f", list, "-P", "bytes=10", b}, {"-Q", a}, {"--nosuch", a}, {"--brief=1", a}, {"-e"},
		{"--ex", "json", a}, {"-e", "nosuch", a}, {"-P", "nosuch=1", a}, {"-P", "indir=99999", a},
		{link}, // -z and the other refused non-goals are TestRefusedFlags' {"-L", link}, {"--dereference", link},
		{"--no-dereference", "-L", link}, {"-L", "--no-dereference", link}, {}, {"-b"},
		{"-f", odd}, {"-P", "regex=-9223372036854775809", a}, {"-P", "regex=-9223372036854775808", a},
		{"-P", "regex=9223372036854775808", a}, {"-P", "regex=-4294967297", a},
	}
	for _, f := range forms {
		compareForm(t, ref, f, "")
	}
	for _, f := range [][]string{{link}, {"-h", link}, {a, "-b"}, {"-b", a}} {
		compareForm(t, ref, f, "1")
		compareForm(t, ref, f, "-") // set and empty: getenv still finds it
	}
}

// compareForm runs one argument vector through both programs, with
// POSIXLY_CORRECT set to posixly ("-" for set and empty) unless it is "".
func compareForm(t *testing.T, ref string, args []string, posixly string) {
	t.Helper()
	switch posixly {
	case "":
		_ = os.Unsetenv("POSIXLY_CORRECT")
	case "-":
		t.Setenv("POSIXLY_CORRECT", "")
	default:
		t.Setenv("POSIXLY_CORRECT", posixly)
	}
	defer func() { _ = os.Unsetenv("POSIXLY_CORRECT") }()
	wantOut, wantErr, wantCode := runReference(t, ref, args)
	gotOut, gotErr, gotCode := capture(t, "", args...)
	name := strings.Join(args, " ")
	if posixly != "" {
		name = "POSIXLY_CORRECT " + name
	}
	if gotOut != wantOut || gotCode != wantCode {
		t.Errorf("[%s]\n  softmagic: exit %d %q\n  reference: exit %d %q", name, gotCode, gotOut, wantCode, wantOut)
	}
	if (gotErr == "") != (wantErr == "") {
		t.Errorf("[%s] stderr: softmagic %q, reference %q", name, gotErr, wantErr)
	}
}

// TestCheckOutput: -c prints what file -c prints for the library's whole
// Magdir: the warning, the column header and every rule line's parsed
// form, on standard error, byte for byte, with the same exit code.
func TestCheckOutput(t *testing.T) {
	ref := referenceDir(t)
	magdir := filepath.Join("..", "..", "..", "softmagic", "magic", "Magdir")
	if _, err := os.Stat(magdir); err != nil {
		t.Skip("library Magdir not beside this checkout")
	}
	args := []string{"-m", magdir, "-c"}
	wantOut, wantErr, wantCode := runReferenceRaw(t, ref, args)
	gotOut, gotErr, gotCode := capture(t, "", args...)
	if gotOut != wantOut || gotErr != wantErr || gotCode != wantCode {
		t.Errorf("-c differs: exit %d/%d, stdout %d/%d bytes, stderr %d/%d bytes",
			gotCode, wantCode, len(gotOut), len(wantOut), len(gotErr), len(wantErr))
	}
}
