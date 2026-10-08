// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// referenceModes are the output modes compared against the reference. Each
// runs over every file, with names, so the stat layer and the column padding
// are compared along with the answers. The -k modes list every match, and the
// last two are the flags azul passes to libmagic for its two decoders.
var referenceModes = [][]string{
	{},
	{"-b"},
	{"-i"},
	{"--mime-type"},
	{"--mime-encoding"},
	{"--extension"},
	{"--apple"},
	{"-L"},
	{"-N"},
	{"-E"},
	{"-r"},
	{"-k"},
	{"-k", "-i"},
	{"-k", "--mime-type"},
	{"-k", "--mime-encoding"},
	{"-k", "--extension"},
	{"-k", "--apple"},
	{"-k", "-r", "-E"},
	{"-k", "-r", "-E", "--mime-type"},
	{"-e", "soft"},
	{"-e", "text"},
	{"-e", "ascii"},
	{"-e", "encoding"},
	{"-e", "tar"},
	{"-e", "json"},
	{"-e", "csv"},
	{"-e", "simh"},
	{"-e", "cdf"},
	{"-e", "elf"},
	{"-e", "compress"},
	{"-e", "apptype"},
	{"-e", "tokens"},
	{"-i", "-e", "encoding"},
	{"-i", "-e", "text"},
	{"--extension", "-e", "soft"},
	{"-k", "-e", "text"},
	{"-k", "-i", "-e", "soft"},
	{"-k", "-e", "json", "-e", "encoding"},
	{"--exclude-quiet", "nosuch", "-e", "elf"},
	{"-P", "indir=0"},
	{"-P", "indir=1"},
	{"-P", "name=1"},
	{"-P", "name=2"},
	{"-P", "regex=0"},
	{"-P", "regex=16"},
	{"-P", "encoding=0"},
	{"-P", "encoding=16"},
	{"-P", "elf_notes=1"},
	{"-P", "elf_phnum=3"},
	{"-P", "elf_shnum=5"},
	{"-P", "elf_shsize=100"},
	{"-P", "bytes=0"},
	{"-P", "bytes=100"},
	{"-P", "magwarn=0"},
	{"-P", "b=50", "-P", "in=2"},
	{"-k", "-P", "name=1"},
	{"-k", "-P", "indir=1"},
	{"-i", "-P", "indir=1"},
	{"--mime-type", "-P", "name=1"},
	{"--mime-encoding", "-P", "name=1"},
	{"--extension", "-P", "indir=1"},
	{"--apple", "-P", "name=1"},
	{"-k", "-i", "-P", "elf_shsize=100"},
}

// referenceBatch is how many files go to one invocation. The padding of the
// answers depends on the longest name in the invocation, which is the same
// for both programs, so batching changes nothing that is compared.
const referenceBatch = 256

// maxReported caps the differences printed; the count is always reported.
const maxReported = 20

// TestReference runs softmagic and the reference file(1) side by side over the
// same files in every mode of referenceModes, and requires stdout, stderr and
// the exit code to be byte-identical. Unlike TestCorpus it does not depend on
// stored expectations, so it covers any file it is pointed at.
//
// The reference is built by scripts/reference.sh (make reference) into
// .reference at the repository root, or named by SOFTMAGIC_REFERENCE (a
// directory holding file and magic.mgc); without one the test skips. The
// inputs are the library corpus beside this checkout, if present, plus every
// path under the colon-separated directories in SOFTMAGIC_REFERENCE_DIRS.
func TestReference(t *testing.T) {
	ref := referenceDir(t)
	files := referenceInputs(t)
	if len(files) == 0 {
		t.Skip("no inputs: neither the library corpus nor SOFTMAGIC_REFERENCE_DIRS")
	}
	t.Logf("%d files, each in %d modes", len(files), len(referenceModes))
	for _, mode := range referenceModes {
		// One subtest per mode, named by its flags, so a run can select
		// modes: go test -run 'TestReference/-k'.
		t.Run("["+strings.Join(mode, " ")+"]", func(t *testing.T) {
			var diffs []string
			for start := 0; start < len(files); start += referenceBatch {
				batch := files[start:min(start+referenceBatch, len(files))]
				diffs = append(diffs, compareBatch(t, ref, mode, batch)...)
			}
			for i, d := range diffs {
				if i == maxReported {
					t.Errorf("... and %d more", len(diffs)-maxReported)
					break
				}
				t.Error(d)
			}
			if len(diffs) > 0 {
				t.Logf("%d differences", len(diffs))
			}
		})
	}
}

// referenceDir finds the reference build, or skips.
func referenceDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("SOFTMAGIC_REFERENCE")
	if dir == "" {
		// The test runs in cmd/softmagic; the build is at the repository root.
		dir = filepath.Join("..", "..", ".reference")
	}
	for _, name := range []string{"file", "magic.mgc"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Skipf("no reference file(1) in %s: run make reference", dir)
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// referenceInputs lists the paths to compare, sorted: the corpus's sample
// files and every path, of any type, under SOFTMAGIC_REFERENCE_DIRS. A
// directory that cannot be read is still listed itself, since what both
// programs say about it is part of the comparison.
func referenceInputs(t *testing.T) []string {
	t.Helper()
	var files []string
	corpus := filepath.Join("..", "..", "..", "softmagic", "testdata", "corpus")
	if matches, err := filepath.Glob(filepath.Join(corpus, "*.testfile")); err == nil {
		files = append(files, matches...)
	}
	for _, root := range filepath.SplitList(os.Getenv("SOFTMAGIC_REFERENCE_DIRS")) {
		if root == "" {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if d != nil && isCharDevice(path, d) && volatile(path) {
				t.Logf("skipped %s: its content changes from read to read", path)
				return nil
			}
			files = append(files, path)
			if err != nil {
				return fs.SkipDir
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	sort.Strings(files)
	return files
}

// isCharDevice reports whether path is a character device or a symbolic
// link to one: the modes that read through links read the device.
func isCharDevice(path string, d fs.DirEntry) bool {
	if d.Type()&fs.ModeCharDevice != 0 {
		return true
	}
	if d.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode()&fs.ModeCharDevice != 0
}

// volatile reports whether two reads of a device return different bytes, as
// /dev/random's do. The two programs each read it once, so they identify
// different data and cannot be compared. The reads are non-blocking, so a
// terminal with nothing to read is stable, not a hang.
func volatile(path string) bool {
	var first []byte
	for i := range 2 { // attempt
		f, err := openForRead(path) // openedFile
		if err != nil {
			return false
		}
		buf := make([]byte, 4096)
		n, rerr := readOnce(f, buf) // bytesRead
		if cerr := f.Close(); cerr != nil || rerr != nil {
			return false
		}
		if i == 0 {
			first = buf[:n]
		} else if !bytes.Equal(first, buf[:n]) {
			return true
		}
	}
	return false
}

// compareBatch runs both programs over one batch in one mode and describes
// every difference: the exit code, stderr, and each stdout line that differs.
func compareBatch(t *testing.T, ref string, mode, batch []string) []string {
	t.Helper()
	args := append(append([]string{}, mode...), batch...)
	wantOut, wantErr, wantCode := runReference(t, ref, args)
	gotOut, gotErr, gotCode := capture(t, "", args...)

	flags := strings.Join(mode, " ")
	var diffs []string
	if gotCode != wantCode {
		diffs = append(diffs, fmt.Sprintf("[%s] exit %d, reference %d (batch from %s)",
			flags, gotCode, wantCode, batch[0]))
	}
	if gotErr != wantErr {
		diffs = append(diffs, fmt.Sprintf("[%s] stderr differs (batch from %s)\n  softmagic: %s\n  reference: %s",
			flags, batch[0], gotErr, wantErr))
	}
	if gotOut == wantOut {
		return diffs
	}
	got, want := strings.Split(gotOut, "\n"), strings.Split(wantOut, "\n")
	for i := range max(len(got), len(want)) {
		g, w := lineAt(got, i), lineAt(want, i)
		if g != w {
			diffs = append(diffs, fmt.Sprintf("[%s]\n  softmagic: %s\n  reference: %s", flags, g, w))
		}
	}
	return diffs
}

// runReference runs the reference file(1) with its own database.
func runReference(t *testing.T, ref string, args []string) (string, string, int) {
	t.Helper()
	return runReferenceRaw(t, ref, append([]string{"-m", filepath.Join(ref, "magic.mgc")}, args...))
}

// runReferenceRaw runs the reference file(1) with exactly these arguments.
func runReferenceRaw(t *testing.T, ref string, args []string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(ref, "file"), args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running the reference: %v", err)
		}
		code = exit.ExitCode()
	}
	return out.String(), errb.String(), code
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<missing>"
}
