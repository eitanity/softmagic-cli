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
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process: it returns the exit code.
func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts, names, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if opts.version {
		printVersion(stdout)
		return 0
	}
	if opts.help {
		usage(stdout)
		return 0
	}
	db, err := loadDatabase(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
		return 1
	}
	d := newDriver(db, opts, stdin, stdout)
	status := 0
	if opts.namesFrom != "" {
		fromFile, err := readNames(opts.namesFrom, stdin)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "softmagic: %v\n", err)
			return 1
		}
		if !d.processNames(fromFile, true) {
			status = 1
		}
	} else if len(names) == 0 {
		usage(stderr)
		return 1
	}
	if !d.processNames(names, false) {
		status = 1
	}
	if !d.flush() {
		status = 1
	}
	return status
}
