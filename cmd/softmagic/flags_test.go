// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eitanity/softmagic"
)

// TestParams: -P as file(1)'s setparam reads it. The name selects the first
// parameter it begins, the value is read as atoi reads it, and 0 reaches
// the library as -1, which it takes as 0 rather than as its default.
func TestParams(t *testing.T) {
	for _, c := range []struct { // testCase
		args  []string
		bytes int
		lim   softmagic.Limits
		err   string
	}{
		{args: []string{"-P", "bytes=10"}, bytes: 10},
		{args: []string{"-P", "b=10"}, bytes: 10},
		{args: []string{"-P", "elf=3"}, lim: softmagic.Limits{ELFNotes: 3}},
		{args: []string{"-P", "in=0"}, lim: softmagic.Limits{Indirect: -1}},
		{args: []string{"-P", "name=7x", "-P", "regex= 12"}, lim: softmagic.Limits{Name: 7, Regex: 12}},
		{args: []string{"-P", "indir=abc"}, lim: softmagic.Limits{Indirect: -1}},
		{args: []string{"-P", "bytes=0"}, bytes: -1},
		{args: []string{"-P", "magwarn=5"}},
		{args: []string{"-P", "indir=70000"}, err: "Out of bounds value 70000 for indir"},
		{args: []string{"-P", "indir=-1"}, err: "Out of bounds value -1 for indir"},
		{args: []string{"-P", "regex=99999999999"}, err: "Out of bounds value 1215752191 for regex"},
		{args: []string{"-P", "regex=999999999999999999999"}, err: "Out of bounds value -1 for regex (Numerical result out of range)"},
		{args: []string{"-P", "regex=4294967296"}, lim: softmagic.Limits{Regex: -1}},
		{args: []string{"-P", "foo=1"}, err: "Unknown param foo=1"},
		{args: []string{"-P", "bytes"}, err: "Unknown param bytes"},
	} {
		o, err := parseArgs(append(c.args, "x")) // parsedOptions
		switch {
		case c.err != "":
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%v: error %v, want %q", c.args, err, c.err)
			}
		case err != nil:
			t.Errorf("%v: %v", c.args, err)
		case o.maxBytes != c.bytes || o.limits != c.lim:
			t.Errorf("%v: bytes %d limits %+v, want %d %+v", c.args, o.maxBytes, o.limits, c.bytes, c.lim)
		}
	}
}

// TestExcludeFlags: -e takes file(1)'s names, an unknown one is a usage
// error, and --exclude-quiet ignores what it does not know.
func TestExcludeFlags(t *testing.T) {
	o, err := parseArgs([]string{"-e", "soft", "-e", "ascii", "--exclude-quiet", "nosuch", "x"})
	if err != nil || o.exclude != softmagic.CheckSoft|softmagic.CheckText {
		t.Fatalf("exclude %v, err %v", o.exclude, err)
	}
	if _, err := parseArgs([]string{"-e", "nosuch", "x"}); err == nil || !strings.Contains(err.Error(), "Usage:") {
		t.Fatalf("-e nosuch: %v", err)
	}
}

// TestRefusedFlags: the non-goals are refused by name, with exit 1.
func TestRefusedFlags(t *testing.T) {
	for _, flag := range []string{"-z", "-Z", "-C", "-d", "--uncompress", "--uncompress-noreport", "--compile", "--debug"} {
		if _, errOut, code := capture(t, "", flag, "x"); code != 1 || !strings.Contains(errOut, flag) {
			t.Errorf("%s: exit %d, stderr %q", flag, code, errOut)
		}
	}
	d := t.TempDir()
	f := filepath.Join(d, "txt")
	writeFile(t, f, "hello\n")
	if out, _, code := capture(t, "", "-S", "-b", f); code != 0 || out != "ASCII text\n" {
		t.Errorf("-S: %q exit %d", out, code)
	}
}

// TestListAndCheck: -l prints the database in matching order and ignores
// names; -c loads the rules and prints nothing when they are sound.
func TestListAndCheck(t *testing.T) {
	out, _, code := capture(t, "", "-l", "ignored")
	if code != 0 || !strings.HasPrefix(out, "Set 0:\nBinary patterns:\nStrength = ") {
		t.Fatalf("-l: exit %d, %.80q", code, out)
	}
	if out, errOut, code := capture(t, "", "-c"); code != 0 || out != "" || errOut != "" {
		t.Fatalf("-c: exit %d, %q %q", code, out, errOut)
	}
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "bad"), "0 nosuchtype x bad\n")
	if _, errOut, code := capture(t, "", "-m", d, "-c"); code != 1 || errOut == "" {
		t.Fatalf("-c on a bad rule: exit %d, %q", code, errOut)
	}
}

// TestPreserveAtime: -p puts back the access time reading changed.
func TestPreserveAtime(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "txt") // textFile
	writeFile(t, f, "hello\n")
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}
	if _, _, code := capture(t, "", "-p", f); code != 0 {
		t.Fatalf("-p: exit %d", code)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	atime, ok := accessTime(info)
	if !ok {
		t.Skip("this platform does not report access times")
	}
	if !atime.Equal(old) || !info.ModTime().Equal(old) {
		t.Errorf("-p: atime %v mtime %v, want %v", atime, info.ModTime(), old)
	}
}

// TestJSONEveryName: --json prints one JSON object per line and nothing
// else, for library answers, stat-layer answers and errors alike.
func TestJSONEveryName(t *testing.T) {
	d := t.TempDir()             // tempDir
	f := filepath.Join(d, "txt") // textFile
	writeFile(t, f, "hello\n")
	missing := filepath.Join(d, "missing")
	out, _, _ := capture(t, "", "--json", f, d, missing)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %q", out)
	}
	var recs [3]jsonLine
	for i, l := range lines {
		if err := json.Unmarshal([]byte(l), &recs[i]); err != nil {
			t.Fatalf("line %d is not JSON: %q (%v)", i, l, err)
		}
	}
	if recs[0].Name != f || recs[0].Result == nil || recs[0].Result.Description != "ASCII text" {
		t.Errorf("file: %+v", recs[0])
	}
	if recs[1].Name != d || recs[1].Answer != "directory" || recs[1].Result != nil {
		t.Errorf("directory: %+v", recs[1])
	}
	if recs[2].Name != missing || !strings.Contains(recs[2].Answer+recs[2].Error, "No such file") {
		t.Errorf("missing: %+v", recs[2])
	}
}

// parseArgs is parsePlan's final options and first error, for the tests
// of single options.
func parseArgs(args []string) (options, error) {
	p := parsePlan(args, false) // parsedPlan
	switch {
	case p.fatal != "":
		return p.final, errors.New(p.fatal)
	case len(p.errs) > 0:
		return p.final, fmt.Errorf("softmagic: %s\n%s", p.errs[0], usageText)
	}
	return p.final, nil
}
