// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

// Command softmagic is file(1) over the softmagic library: the same output,
// byte for byte, for -b, -i, --mime-type, --mime-encoding, --extension and
// --apple, the same exit codes, plus --json, which prints the library's
// Result so what was examined is visible.
//
// It is a separate module from the library, which
// itself never prints, parses flags or touches the file system.
package main

import (
	"fmt"
	"github.com/eitanity/softmagic"
	"github.com/eitanity/softmagic/compile"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process: it returns the exit code.
func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	_, posixly := os.LookupEnv("POSIXLY_CORRECT")        // set at all counts, as getenv sees it
	p := parsePlan(args, posixly)                        // plan
	db, status, ok := runLists(p, stdin, stdout, stderr) // listsOK
	if !ok {
		return 1
	}
	if code, stop := stopEarly(p, stdout, stderr); stop {
		return code
	}
	opts := p.final
	if opts.check {
		return checkRules(opts, stderr)
	}
	if db == nil {
		var err error
		if db, err = loadDatabase(opts); err != nil {
			_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
			return 1
		}
	}
	switch {
	case opts.list:
		_, _ = fmt.Fprint(stdout, db.List())
		return 0
	case len(p.names) == 0:
		if len(p.lists) == 0 {
			usage(stderr)
			return 1
		}
		return status
	}
	d := newDriver(db, opts, stdin, stdout)
	ok = d.processNames(p.names, false)
	if !d.flush() || !ok { // flush first: a failed name still printed its line
		status = 1
	}
	return status
}

// runLists reads each -f list where it appears among the options, as file
// does, loading the database the first time. It returns the database (nil
// when there was no list), the exit status so far, and false when the
// command cannot go on.
func runLists(p plan, stdin *os.File, stdout, stderr *os.File) (*softmagic.Database, int, bool) {
	var db *softmagic.Database // database
	status := 0
	for _, job := range p.lists {
		if db == nil {
			var err error
			if db, err = loadDatabase(job.opts); err != nil {
				_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
				return nil, 1, false
			}
		}
		names, err := readNames(job.from, stdin)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
			return nil, 1, false
		}
		d := newDriver(db, job.opts, stdin, stdout)
		ok := d.processNames(names, true)
		if !d.flush() || !ok {
			status = 1
		}
	}
	return db, status, true
}

// stopEarly is what ends the command after the options: --version or
// --help where they appeared, an option that ends it, or usage errors.
func stopEarly(p plan, stdout, stderr *os.File) (int, bool) { // plan
	switch {
	case p.stop == "version":
		printVersion(stdout)
		return 0, true
	case p.stop == "help":
		help(stdout)
		return 0, true
	case p.fatal != "":
		_, _ = fmt.Fprintln(stderr, p.fatal)
		return 1, true
	case len(p.errs) > 0:
		for _, e := range p.errs {
			_, _ = fmt.Fprintf(stderr, "softmagic: %s\n", e)
		}
		usage(stderr)
		return 1, true
	}
	return 0, false
}

// checkRules is -c: for each directory -m (and --extra-magic) names, the
// reference's warning that it is reading rule source, its column header,
// and each rule line in libmagic's parsed form, all on standard error.
// The embedded database is compiled already, as a .mgc file is, and file
// prints nothing for that. A rule the library refuses is reported after
// the lines before it, and the exit status is 1.
func checkRules(o options, stderr *os.File) int {
	status := 0
	for _, dir := range append(splitDirs(o.magicDirs), splitDirs(o.extraDirs)...) {
		_, _ = fmt.Fprintf(stderr, "Warning: using regular magic file `%s'\n", dir)
		_, _ = fmt.Fprintln(stderr, "cont\toffset\ttype\topcode\tmask\tvalue\tdesc")
		out, err := compile.Dump(os.DirFS(dir), softmagic.CompileOptions{SourceDir: dir})
		_, _ = fmt.Fprint(stderr, out)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
			status = 1
		}
	}
	return status
}
