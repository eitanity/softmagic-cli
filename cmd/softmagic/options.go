// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// outputMode is what each line reports, as file(1)'s flags select it.
type outputMode int

const (
	modeDescription  outputMode = iota
	modeMime                    // -i: type; charset=encoding
	modeMimeType                // --mime-type
	modeMimeEncoding            // --mime-encoding
	modeExtension               // --extension
	modeApple                   // --apple
	modeJSON                    // --json
)

// options is the parsed command line.
type options struct {
	mode        outputMode
	brief       bool   // -b
	noPad       bool   // -N
	noBuffer    bool   // -n
	nulSep      int    // -0 once or twice
	separator   string // -F
	errExit     bool   // -E
	followLinks bool   // -L (default off: -h)
	devices     bool   // -s
	raw         bool   // -r
	keepGoing   bool   // -k: every match, libmagic's MAGIC_CONTINUE
	magicDirs   string // -m
	extraDirs   string // --extra-magic
	maxBytes    int    // --max-bytes / -P bytes=
	namesFrom   string // -f
	version     bool
	help        bool
}

const usageText = `Usage: softmagic [-bEhikLnNrs0] [--apple] [--extension] [--mime-encoding]
                 [--mime-type] [--json] [-F <separator>] [-m <magicdirs>]
                 [--extra-magic <dirs>] [--max-bytes <n>] [-P bytes=<n>]
                 [-f <namefile>] <file> ...
       softmagic --version
       softmagic --help
`

func usage(w io.Writer) { _, _ = fmt.Fprint(w, usageText) }

// parseArgs reads file(1)'s options. Short flags take one letter each; the
// combined form (-bi) is not accepted.
func parseArgs(args []string) (options, []string, error) {
	var o options
	fs := flag.NewFlagSet("softmagic", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var mime, mimeType, mimeEncoding, extension, apple, jsonOut, uncompress bool
	var param string
	fs.BoolVar(&o.brief, "b", false, "")
	fs.BoolVar(&mime, "i", false, "")
	fs.BoolVar(&mime, "mime", false, "")
	fs.BoolVar(&mimeType, "mime-type", false, "")
	fs.BoolVar(&mimeEncoding, "mime-encoding", false, "")
	fs.BoolVar(&extension, "extension", false, "")
	fs.BoolVar(&apple, "apple", false, "")
	fs.BoolVar(&jsonOut, "json", false, "")
	fs.BoolVar(&o.noPad, "N", false, "")
	fs.BoolVar(&o.noBuffer, "n", false, "")
	fs.BoolVar(&o.errExit, "E", false, "")
	fs.BoolVar(&o.followLinks, "L", false, "")
	var noFollow bool
	fs.BoolVar(&noFollow, "h", false, "")
	fs.BoolVar(&o.devices, "s", false, "")
	fs.BoolVar(&o.raw, "r", false, "")
	fs.BoolVar(&o.keepGoing, "k", false, "")
	fs.BoolVar(&uncompress, "z", false, "")
	fs.StringVar(&o.separator, "F", ":", "")
	fs.StringVar(&o.magicDirs, "m", "", "")
	fs.StringVar(&o.extraDirs, "extra-magic", "", "")
	fs.IntVar(&o.maxBytes, "max-bytes", 0, "")
	fs.StringVar(&param, "P", "", "")
	fs.StringVar(&o.namesFrom, "f", "", "")
	fs.BoolVar(&o.version, "version", false, "")
	fs.BoolVar(&o.version, "v", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	var nul0, nul00 bool
	fs.BoolVar(&nul0, "0", false, "")
	fs.BoolVar(&nul00, "00", false, "")
	if err := fs.Parse(args); err != nil {
		return o, nil, fmt.Errorf("softmagic: %v\n%s", err, usageText)
	}
	if uncompress {
		return o, nil, errors.New("softmagic: -z (look inside compressed files) is not supported: it is a non-goal of the library")
	}
	o.mode = selectMode(mime, mimeType, mimeEncoding, extension, apple, jsonOut)
	if nul00 {
		o.nulSep = 2
	} else if nul0 {
		o.nulSep = 1
	}
	if noFollow {
		o.followLinks = false
	}
	if param != "" {
		n, err := parseParam(param)
		if err != nil {
			return o, nil, err
		}
		o.maxBytes = n
	}
	return o, fs.Args(), nil
}

func selectMode(mime, mimeType, mimeEncoding, extension, apple, jsonOut bool) outputMode {
	switch {
	case jsonOut:
		return modeJSON
	case apple:
		return modeApple
	case extension:
		return modeExtension
	case mime || (mimeType && mimeEncoding):
		return modeMime
	case mimeType:
		return modeMimeType
	case mimeEncoding:
		return modeMimeEncoding
	default:
		return modeDescription
	}
}

// parseParam accepts file(1)'s -P bytes=N; the other parameters are
// limits the library fixes.
func parseParam(p string) (int, error) {
	name, value, ok := strings.Cut(p, "=")
	if !ok || name != "bytes" {
		return 0, fmt.Errorf("softmagic: unsupported parameter `%s' (only bytes=N)", p)
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("softmagic: bad value for bytes: `%s'", value)
	}
	return n, nil
}

// readNames is -f: one file name per line, "-" for standard input.
func readNames(from string, stdin io.Reader) ([]string, error) {
	r := stdin
	if from != "-" {
		f, err := os.Open(from) // the list file the user named; see the Makefile on G304
		if err != nil {
			return nil, fmt.Errorf("softmagic: cannot open `%s' (%s)", from, cerror(err))
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	var names []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			names = append(names, line)
		}
	}
	return names, sc.Err()
}
