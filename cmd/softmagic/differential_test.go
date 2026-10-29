// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// differentialModes are the modes each case is compared in.
var differentialModes = [][]string{{"-b"}, {"-b", "-i"}}

// diffCase is one case the library's TestExportCorpus wrote: a directory
// holding input, the rules in magic unless the case uses the stock
// database, and extra flags one per line in flags.
type diffCase struct {
	name  string // <target>/<entry>
	input string
	magic string
	flags []string
}

// TestDifferential runs softmagic and the reference over every case under
// SOFTMAGIC_DIFF_DIR, the cases the library exports from its fuzz corpora
// (make differential), and requires stdout, stderr and the exit code to be
// byte-identical in each of differentialModes. Every difference is listed,
// grouped by what it looks like and with the case's directory, in the file
// SOFTMAGIC_DIFF_REPORT names when it is set. It needs the reference, as
// TestReference does, and skips without SOFTMAGIC_DIFF_DIR.
func TestDifferential(t *testing.T) {
	dir := os.Getenv("SOFTMAGIC_DIFF_DIR")
	if dir == "" {
		t.Skip("SOFTMAGIC_DIFF_DIR is not set: run make differential")
	}
	ref := referenceDir(t)
	cases := differentialCases(t, dir)
	if len(cases) == 0 {
		t.Fatalf("no cases under %s", dir)
	}
	r := diffReport{perTarget: map[string]int{}, groups: map[string][]string{}}
	for _, c := range cases {
		for _, mode := range differentialModes {
			r.compare(t, ref, c, mode)
		}
	}
	t.Logf("%d cases, each in %d modes", len(cases), len(differentialModes))
	if path := os.Getenv("SOFTMAGIC_DIFF_REPORT"); path != "" {
		r.write(t, path)
	}
	for i, d := range r.lines {
		if i == maxReported {
			t.Errorf("... and %d more", len(r.lines)-maxReported)
			break
		}
		t.Error(d)
	}
	if len(r.lines) > 0 {
		targets := make([]string, 0, len(r.perTarget))
		for name, n := range r.perTarget {
			targets = append(targets, fmt.Sprintf("%s %d", name, n))
		}
		sort.Strings(targets)
		t.Logf("%d differences: %s", len(r.lines), strings.Join(targets, ", "))
	}
}

// diffReport collects the differences: each described in full, counted
// per target, and grouped by diffSignature.
type diffReport struct {
	perTarget map[string]int
	groups    map[string][]string // signature: case names
	lines     []string
}

// compare runs both programs on one case in one mode and records any
// difference.
func (r *diffReport) compare(t *testing.T, ref string, c diffCase, mode []string) {
	t.Helper()
	args := append(append([]string{}, mode...), c.flags...)
	var wantOut, wantErr string
	var wantCode int
	if c.magic != "" {
		args = append(args, "-m", c.magic, c.input)
		wantOut, wantErr, wantCode = runReferenceRaw(t, ref, args)
	} else {
		args = append(args, c.input)
		wantOut, wantErr, wantCode = runReference(t, ref, args)
	}
	gotOut, gotErr, gotCode := capture(t, "", args...)
	if gotOut == wantOut && gotErr == wantErr && gotCode == wantCode {
		return
	}
	r.perTarget[strings.Split(c.name, "/")[0]]++
	sig := diffSignature(filepath.Dir(c.input), gotOut+gotErr, wantOut+wantErr)
	r.groups[sig] = append(r.groups[sig], c.name+" ["+strings.Join(mode, " ")+"]")
	r.lines = append(r.lines, fmt.Sprintf("%s [%s]\n  softmagic: %q %q exit %d\n  reference: %q %q exit %d",
		c.name, strings.Join(args[:len(args)-1], " "), gotOut, gotErr, gotCode, wantOut, wantErr, wantCode))
}

// write writes the groups and then every difference to path.
func (r *diffReport) write(t *testing.T, path string) {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	text := diffGroups(r.groups, len(r.lines)) + "\n" + strings.Join(r.lines, "\n") + "\n"
	err = root.WriteFile(filepath.Base(path), []byte(text), 0o600)
	if cerr := root.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
}

// diffNumber is a run of digits, in hex after 0x.
var diffNumber = regexp.MustCompile(`0x[0-9a-fA-F]+|[0-9]+`)

// diffSignature is what one difference looks like with the case's
// directory, the numbers and all but the first line of each program's
// output taken out, so the differences one cause makes share it.
func diffSignature(dir, got, want string) string {
	short := func(s string) string {
		s, _, _ = strings.Cut(strings.ReplaceAll(s, dir, "<case>"), "\n")
		s = diffNumber.ReplaceAllString(s, "N")
		if len(s) > 120 {
			s = s[:120] + "..."
		}
		return s
	}
	return "softmagic: " + short(got) + "\nreference: " + short(want)
}

// diffGroups lists the signatures, the commonest first, each with how
// many differences have it and a few of their cases.
func diffGroups(groups map[string][]string, total int) string {
	sigs := make([]string, 0, len(groups))
	for sig := range groups {
		sigs = append(sigs, sig)
	}
	sort.Slice(sigs, func(i, j int) bool {
		if len(groups[sigs[i]]) != len(groups[sigs[j]]) {
			return len(groups[sigs[i]]) > len(groups[sigs[j]])
		}
		return sigs[i] < sigs[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%d differences in %d groups\n", total, len(sigs))
	for _, sig := range sigs {
		names := groups[sig]
		fmt.Fprintf(&b, "\n%d× %s\n  e.g. %s\n", len(names), sig, strings.Join(names[:min(3, len(names))], ", "))
	}
	return b.String()
}

// differentialCases lists the cases under dir, sorted by name.
func differentialCases(t *testing.T, dir string) []diffCase {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	names, err := fs.Glob(root.FS(), "*/*")
	if err != nil {
		t.Fatal(err)
	}
	cases := make([]diffCase, 0, len(names))
	for _, name := range names {
		c := diffCase{name: name, input: filepath.Join(dir, name, "input")}
		if _, err := root.Stat(name + "/magic"); err == nil {
			c.magic = filepath.Join(dir, name, "magic")
		}
		if b, err := root.ReadFile(name + "/flags"); err == nil {
			c.flags = strings.Fields(string(b))
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })
	return cases
}
