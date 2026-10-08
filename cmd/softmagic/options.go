// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/eitanity/softmagic"
	"io"
	"math"
	"os"
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
	brief       bool             // -b
	noPad       bool             // -N
	noBuffer    bool             // -n
	nulSep      int              // -0 once or twice
	separator   string           // -F
	errExit     bool             // -E
	followLinks bool             // -L (default off: -h)
	devices     bool             // -s
	raw         bool             // -r
	keepGoing   bool             // -k: every match, libmagic's MAGIC_CONTINUE
	exclude     softmagic.Checks // -e, --exclude-quiet
	limits      softmagic.Limits // -P
	magicDirs   string           // -m
	extraDirs   string           // --extra-magic
	maxBytes    int              // --max-bytes / -P bytes=
	list        bool             // -l: print the database in matching order
	check       bool             // -c: load the rules, which checks them, and stop
	keepAtime   bool             // -p: restore each file's access time after reading it
	// The flags as given, which finish turns into mode, nulSep and brief.
	mime, mimeType, mimeEncoding, extension, apple, jsonOut bool
	briefCount, nulCount                                    int
}

const usageText = `Usage: softmagic [-bcEhikLlNnprsS0] [--apple] [--extension] [--mime-encoding]
                 [--mime-type] [--json] [-e <check>] [-F <separator>] [-f <namefile>]
                 [-m <magicdirs>] [--extra-magic <dirs>] [-P <name>=<value>] <file> ...
       softmagic [-m <magicdirs>] -c
       softmagic [-m <magicdirs>] -l
       softmagic -v | --version
       softmagic --help
`

// helpText is --help: file(1)'s options, then this program's own.
const helpText = `Usage: softmagic [OPTION...] [FILE...]
Determine type of FILEs, as file(1) 5.48 does.

      --help                 display this help and exit
  -v, --version              output version information and exit
  -m, --magic-file LIST      use LIST as a colon-separated list of magic
                               rule directories instead of the built-in database
  -b, --brief                do not prepend filenames to output lines
  -c, --checking-printout    print the parsed form of the rules in -m's
                               directories, as file -c does
  -e, --exclude TEST         exclude TEST from the tests performed: apptype,
                               ascii, cdf, compress, csv, elf, encoding, soft,
                               tar, json, simh, text, tokens
      --exclude-quiet TEST   like exclude, but ignore unknown tests
  -f, --files-from FILE      read the filenames to be examined from FILE
  -F, --separator STRING     use string as separator instead of ':'
  -i, --mime                 output MIME type strings (--mime-type and
                               --mime-encoding)
      --apple                output the Apple CREATOR/TYPE
      --extension            output a slash-separated list of extensions
      --mime-type            output the MIME type
      --mime-encoding        output the MIME encoding
  -k, --keep-going           don't stop at the first match
  -l, --list                 list magic strength
  -L, --dereference          follow symlinks (default if POSIXLY_CORRECT is set)
  -h, --no-dereference       don't follow symlinks (default if POSIXLY_CORRECT is not set)
  -n, --no-buffer            do not buffer output
  -N, --no-pad               do not pad output
  -0, --print0               terminate filenames with ASCII NUL
  -p, --preserve-date        preserve access times on files
  -P, --parameter NAME=VALUE set a limit: bytes, elf_notes, elf_phnum,
                               elf_shnum, elf_shsize, encoding, indir, name,
                               regex, magwarn
  -r, --raw                  don't translate unprintable chars to \ooo
  -s, --special-files        treat special (block/char devices) files as
                               ordinary ones
  -S, --no-sandbox           accepted; there is no sandbox to disable
  -E                         an unreadable file is an error, exit 1

This program's own:
      --json                 one JSON object per line: name and the library's Result
      --extra-magic DIRS     append colon-separated rule directories to the database
      --max-bytes N          as -P bytes=N

Not supported (non-goals of the library): -z, --uncompress; -Z,
--uncompress-noreport; -C, --compile; -d, --debug.
`

func usage(w io.Writer) { _, _ = fmt.Fprint(w, usageText) }

func help(w io.Writer) { _, _ = fmt.Fprint(w, helpText) }

// plan is the command line as file(1) acts on it: the -f lists in the
// order they appear, each with the options in force there, then the names
// with the options in force at the end. What file gives libmagic as flags
// (the output mode, -k, -r, -e, -L, -s, -E, -p, -m) is fixed when it first
// loads the database, at the first -f, so options after that one change
// only what file prints itself (-b, -N, -0, -F, -n) and the -P limits.
type plan struct {
	final options
	lists []listJob
	names []string
	errs  []string // getopt's and -e's errors: usage, after any list already read
	fatal string   // an error that ends the command where it stands, as file_err does
	stop  string   // "version" or "help": file prints it and exits there
}

// listJob is one -f list and the options in force where it appears.
type listJob struct {
	from string
	opts options
}

// parsePlan reads args as file(1) does, getopt_long's way.
func parsePlan(args []string, posixly bool) plan {
	events, names, errs := getoptLong(args, posixly)
	cur := options{separator: ":", followLinks: posixly}
	var frozen *options
	p := plan{errs: errs, names: names} // plan
	for _, ev := range events {         // event
		switch ev.spec.long {
		case "files-from":
			if frozen == nil {
				f := cur
				frozen = &f
			}
			p.lists = append(p.lists, listJob{from: ev.value, opts: withLibFlags(cur, *frozen, true)})
			continue
		case "version", "help":
			p.stop, p.final = ev.spec.long, cur
			return p
		}
		msg, fatal := cur.apply(ev)
		switch {
		case fatal:
			p.fatal, p.final = msg, cur
			return p
		case msg != "":
			p.errs = append(p.errs, msg)
		}
	}
	if frozen != nil {
		cur = withLibFlags(cur, *frozen, false)
	}
	cur.finish(len(names))
	p.final = cur
	return p
}

// withLibFlags is cur with the options file had given libmagic when it
// loaded the database (lib). A list read at its -f also sees the -b count
// as it stands then, before the end-of-options rule for -b given twice.
func withLibFlags(cur, lib options, atList bool) options {
	o := cur // merged
	o.mime, o.mimeType, o.mimeEncoding = lib.mime, lib.mimeType, lib.mimeEncoding
	o.extension, o.apple, o.jsonOut = lib.extension, lib.apple, lib.jsonOut
	o.keepGoing, o.raw, o.exclude = lib.keepGoing, lib.raw, lib.exclude
	o.followLinks, o.devices, o.errExit, o.keepAtime = lib.followLinks, lib.devices, lib.errExit, lib.keepAtime
	o.magicDirs, o.extraDirs = lib.magicDirs, lib.extraDirs
	if atList {
		o.finish(-1)
	}
	return o
}

// apply is one option, as file.c's switch takes it. It returns an error
// message and whether it ends the command at once.
func (o *options) apply(ev optEvent) (string, bool) { // event
	switch ev.spec.long {
	case "magic-file":
		o.magicDirs = ev.value
	case "uncompress":
		return "softmagic: -z, --uncompress (look inside compressed files) is not supported: it is a non-goal of the library", true
	case "uncompress-noreport":
		return "softmagic: -Z, --uncompress-noreport (look only inside compressed files) is not supported: it is a non-goal of the library", true
	case "compile":
		return "softmagic: -C, --compile (write a .mgc file) is not supported: the compiled form is the library's own, made by go generate", true
	case "debug":
		return "softmagic: -d, --debug (libmagic's debug trace) is not supported: Result.Rules in --json names the lines that matched", true
	case "exclude", "exclude-quiet":
		c, ok := checkByName(ev.value)
		switch {
		case ok:
			o.exclude |= c
		case ev.spec.long == "exclude":
			return fmt.Sprintf("unknown check `%s' for -e", ev.value), false
		}
	case "parameter":
		if err := applyParams(o, []string{ev.value}); err != nil {
			return err.Error(), true // file_err: the command ends here
		}
	case "max-bytes":
		n, _ := atoi(ev.value)
		o.maxBytes = n
	case "extra-magic":
		o.extraDirs = ev.value
	case "separator":
		o.separator = ev.value
	default:
		o.applyFlag(ev.spec)
	}
	return "", false
}

// applyFlag is an option without a value.
func (o *options) applyFlag(spec *optSpec) {
	switch spec.long {
	case "brief":
		o.briefCount++
	case "checking-printout":
		o.check = true
	case "mime":
		o.mime = true
	case "apple":
		o.apple = true
	case "extension":
		o.extension = true
	case "mime-type":
		o.mimeType = true
	case "mime-encoding":
		o.mimeEncoding = true
	case "json":
		o.jsonOut = true
	case "keep-going":
		o.keepGoing = true
	case "list":
		o.list = true
	case "dereference":
		o.followLinks = true
	case "no-dereference":
		o.followLinks = false
	case "no-buffer":
		o.noBuffer = true
	case "no-pad":
		o.noPad = true
	case "print0":
		o.nulCount++
	case "preserve-date":
		o.keepAtime = true
	case "raw":
		o.raw = true
	case "special-files":
		o.devices = true
	case "no-sandbox": // there is no sandbox to switch off
	default: // -E, the one option with no long name
		o.errExit = true
	}
}

// finish derives what the counts and flags mean once the options are
// read: the output mode, -0's count, and -b's. Given -b exactly twice,
// file is brief only for one name or none (an undocumented rule it applies
// after the options); names < 0 is a list read before that rule.
func (o *options) finish(names int) {
	o.mode = selectMode(o.mime, o.mimeType, o.mimeEncoding, o.extension, o.apple, o.jsonOut)
	o.nulSep = min(o.nulCount, 2)
	o.brief = o.briefCount > 0
	if o.briefCount == 2 && names >= 0 {
		o.brief = names <= 1
	}
}

// checkNames is file.c's nv[]: the names -e takes, in the reference's order.
var checkNames = []struct {
	name  string
	check softmagic.Checks
}{
	{"apptype", softmagic.CheckAppType},
	{"ascii", softmagic.CheckText},
	{"cdf", softmagic.CheckCDF},
	{"compress", softmagic.CheckCompress},
	{"csv", softmagic.CheckCSV},
	{"elf", softmagic.CheckELF},
	{"encoding", softmagic.CheckEncoding},
	{"soft", softmagic.CheckSoft},
	{"tar", softmagic.CheckTar},
	{"json", softmagic.CheckJSON},
	{"simh", softmagic.CheckSIMH},
	{"text", softmagic.CheckText},
	{"tokens", softmagic.CheckTokens},
}

func checkByName(name string) (softmagic.Checks, bool) {
	for _, n := range checkNames {
		if n.name == name {
			return n.check, true
		}
	}
	return 0, false
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

// paramTable is file.c's pm[]: the -P names in the reference's order, with
// the largest value it accepts (magic_getmaxparam).
var paramTable = []struct {
	name string
	max  int
}{
	{"bytes", 0x7fffffff},
	{"elf_notes", 0xffff},
	{"elf_phnum", 0xffff},
	{"elf_shnum", 0xffff},
	{"elf_shsize", 0xffff},
	{"encoding", 0x7fffffff},
	{"indir", 0xffff},
	{"name", 0xffff},
	{"regex", 0xffff},
	{"magwarn", 0x7fffffff},
}

// applyParams is file(1)'s setparam for each -P, in order: the name before
// '=' selects the first parameter it begins (so "b" is bytes), the value is
// read as atoi reads it, and a value out of range, or no parameter, is an
// error. The library takes 0 to mean its default, so a 0 from the command
// line is passed as -1, which it takes as 0.
func applyParams(o *options, params []string) error { // opts
	for _, p := range params {
		key, value, ok := strings.Cut(p, "=")
		i := paramIndex(key) // paramIdx
		if !ok || i < 0 {
			return fmt.Errorf("softmagic: Unknown param %s", p)
		}
		v, saturated := atoi(value) // paramValue
		if v < 0 || v > paramTable[i].max {
			msg := fmt.Sprintf("softmagic: Out of bounds value %d for %s", v, paramTable[i].name)
			if saturated {
				msg += " (Numerical result out of range)" // file_err adds strtol's ERANGE
			}
			return errors.New(msg)
		}
		if v == 0 {
			v = -1
		}
		setParam(o, paramTable[i].name, v)
	}
	return nil
}

func paramIndex(key string) int {
	for i, p := range paramTable {
		if strings.HasPrefix(p.name, key) {
			return i
		}
	}
	return -1
}

func setParam(o *options, name string, v int) { // paramValue
	l := &o.limits // limits
	switch name {
	case "bytes":
		o.maxBytes = v
	case "elf_notes":
		l.ELFNotes = v
	case "elf_phnum":
		l.ELFPhnum = v
	case "elf_shnum":
		l.ELFShnum = v
	case "elf_shsize":
		l.ELFShsize = v
	case "encoding":
		l.Encoding = v
	case "indir":
		l.Indirect = v
	case "name":
		l.Name = v
	case "regex":
		l.Regex = v
	default: // magwarn: warnings while loading rules, which this program does not print
	}
}

// atoi is glibc's atoi: optional blanks and sign, then digits, and nothing
// after them matters; no digits is 0. It is strtol cast to int, so a value
// past the long range saturates first (and reports it, as strtol's ERANGE
// does), and the cast keeps the low 32 bits.
func atoi(s string) (int, bool) { // numberText
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	// The magnitude strtol allows: LONG_MAX, or one more for a negative.
	limit := uint64(math.MaxInt64)
	if neg {
		limit++
	}
	var mag uint64
	saturated := false
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		d := uint64(s[i] - '0')
		if mag > (limit-d)/10 {
			mag, saturated = limit, true // strtol saturates and sets ERANGE
			break
		}
		mag = mag*10 + d
	}
	low := mag & 0xffffffff // the int cast keeps the low 32 bits of the long
	if neg {
		low = -low & 0xffffffff
	}
	v := int64(low)
	if v >= 1<<31 {
		v -= 1 << 32
	}
	return int(v), saturated
}

// readNames is -f: one file name per line, "-" for standard input.
func readNames(from string, stdin io.Reader) ([]string, error) {
	r := stdin // listReader
	if from != "-" {
		f, err := os.Open(from) // the list file the user named; see the Makefile on G304
		if err != nil {
			return nil, fmt.Errorf("softmagic: cannot open `%s' (%s)", from, cerror(err))
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	return splitNames(bufio.NewReader(r))
}

// splitNames is unwrap's getline loop: a name per line, its newline
// removed and nothing else (a carriage return stays, an empty line is an
// empty name), any length, and, as the name is a C string, ending at a NUL.
func splitNames(r *bufio.Reader) ([]string, error) {
	var names []string
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			if nul := strings.IndexByte(line, 0); nul >= 0 {
				line = line[:nul]
			}
			names = append(names, line)
		}
		switch {
		case errors.Is(err, io.EOF):
			return names, nil
		case err != nil:
			return names, err
		}
	}
}
