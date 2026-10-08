// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"strings"
)

// optSpec is one option: file.c's OPTSTRING letter and file_opts.h long
// name, or one of this program's own, which file does not have.
type optSpec struct {
	long  string
	short byte // 0 for a long-only option
	arg   bool
	own   bool // this program's, not file's
}

// optSpecs are file 5.48's options in file_opts.h's order, then this
// program's. The short letters are OPTSTRING "bcCde:Ef:F:hiklLm:nNpP:rsSvzZ0".
var optSpecs = []optSpec{
	{long: "help"},
	{long: "version", short: 'v'},
	{long: "magic-file", short: 'm', arg: true},
	{long: "uncompress", short: 'z'},
	{long: "uncompress-noreport", short: 'Z'},
	{long: "brief", short: 'b'},
	{long: "checking-printout", short: 'c'},
	{long: "exclude", short: 'e', arg: true},
	{long: "exclude-quiet", arg: true},
	{long: "files-from", short: 'f', arg: true},
	{long: "separator", short: 'F', arg: true},
	{long: "mime", short: 'i'},
	{long: "apple"},
	{long: "extension"},
	{long: "mime-type"},
	{long: "mime-encoding"},
	{long: "keep-going", short: 'k'},
	{long: "list", short: 'l'},
	{long: "dereference", short: 'L'},
	{long: "no-dereference", short: 'h'},
	{long: "no-buffer", short: 'n'},
	{long: "no-pad", short: 'N'},
	{long: "print0", short: '0'},
	{long: "preserve-date", short: 'p'},
	{long: "parameter", short: 'P', arg: true},
	{long: "raw", short: 'r'},
	{long: "special-files", short: 's'},
	{long: "no-sandbox", short: 'S'},
	{long: "compile", short: 'C'},
	{long: "debug", short: 'd'},
	{short: 'E'}, // -E has no long name
	{long: "json", own: true},
	{long: "max-bytes", arg: true, own: true},
	{long: "extra-magic", arg: true, own: true},
}

// optEvent is one option as given, in order: the option and its value.
type optEvent struct {
	spec  *optSpec
	value string
}

// getoptLong reads args as glibc's getopt_long reads them for file(1):
// short options cluster (-bik) and take their value attached or as the next
// argument; a long option may be abbreviated to a unique prefix and takes
// "=value" or the next argument; options and names may be mixed, unless
// posixly (POSIXLY_CORRECT), when the first name ends the options; "--"
// ends them too. Errors are getopt's messages, in order.
func getoptLong(args []string, posixly bool) (events []optEvent, names, errs []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return events, append(names, args[i+1:]...), errs
		case a == "-" || !strings.HasPrefix(a, "-"):
			if posixly {
				return events, append(names, args[i:]...), errs
			}
			names = append(names, a)
		case strings.HasPrefix(a, "--"):
			ev, used, err := longOption(a[2:], args[i+1:])
			i += used
			if err != "" {
				errs = append(errs, err)
				continue
			}
			events = append(events, ev)
		default:
			evs, used, errl := shortOptions(a[1:], args[i+1:])
			i += used
			events, errs = append(events, evs...), append(errs, errl...)
		}
	}
	return events, names, errs
}

// longOption reads "--name[=value]"; rest is what follows it, for a value
// given as the next argument. It returns how many of rest it used.
func longOption(text string, rest []string) (optEvent, int, string) {
	name, value, hasValue := strings.Cut(text, "=")
	spec, err := lookupLong(name)
	if err != "" {
		return optEvent{}, 0, err
	}
	switch {
	case !spec.arg && hasValue:
		return optEvent{}, 0, fmt.Sprintf("option '--%s' doesn't allow an argument", spec.long)
	case !spec.arg || hasValue:
		return optEvent{spec: spec, value: value}, 0, ""
	case len(rest) > 0:
		return optEvent{spec: spec, value: rest[0]}, 1, ""
	default:
		return optEvent{}, 0, fmt.Sprintf("option '--%s' requires an argument", spec.long)
	}
}

// lookupLong is getopt_long's matching: an exact name, else a unique
// prefix. This program's own options are matched only when none of file's
// is, so they never make one of file's abbreviations ambiguous (--ext is
// still --extension).
func lookupLong(name string) (*optSpec, string) {
	for i := range optSpecs {
		if optSpecs[i].long != "" && optSpecs[i].long == name {
			return &optSpecs[i], ""
		}
	}
	for _, own := range []bool{false, true} {
		var hits []*optSpec
		for i := range optSpecs {
			s := &optSpecs[i]
			if s.own == own && s.long != "" && strings.HasPrefix(s.long, name) {
				hits = append(hits, s)
			}
		}
		switch {
		case len(hits) == 1:
			return hits[0], ""
		case len(hits) > 1:
			msg := fmt.Sprintf("option '--%s' is ambiguous; possibilities:", name)
			for _, h := range hits {
				msg += " '--" + h.long + "'"
			}
			return nil, msg
		}
	}
	return nil, fmt.Sprintf("unrecognized option '--%s'", name)
}

// shortOptions reads a cluster of short options ("bik" from "-bik"). An
// option that takes a value takes the rest of the cluster, or else the next
// argument. It returns how many of rest it used.
func shortOptions(cluster string, rest []string) ([]optEvent, int, []string) {
	var events []optEvent
	var errs []string
	for j := 0; j < len(cluster); j++ {
		spec := lookupShort(cluster[j])
		switch {
		case spec == nil:
			errs = append(errs, fmt.Sprintf("invalid option -- '%c'", cluster[j]))
		case !spec.arg:
			events = append(events, optEvent{spec: spec})
		case j+1 < len(cluster):
			return append(events, optEvent{spec: spec, value: cluster[j+1:]}), 0, errs
		case len(rest) > 0:
			return append(events, optEvent{spec: spec, value: rest[0]}), 1, errs
		default:
			return events, 0, append(errs, fmt.Sprintf("option requires an argument -- '%c'", cluster[j]))
		}
	}
	return events, 0, errs
}

func lookupShort(c byte) *optSpec {
	for i := range optSpecs {
		if optSpecs[i].short != 0 && optSpecs[i].short == c {
			return &optSpecs[i]
		}
	}
	return nil
}
