// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eitanity/softmagic"
)

// pipeBuf is PIPE_BUF: the reference stops reading a pipe at the first
// short read.
const pipeBuf = 4096

// driver is the per-run state: the database, the options, the output and
// the read buffer, allocated once.
type driver struct {
	db    *softmagic.Database
	o     options
	stdin *os.File
	w     *bufio.Writer
	buf   []byte
	err   error // the first output error; the reference's haderror
}

// out writes to standard output, keeping the first error.
func (d *driver) out(s string) {
	if d.err == nil {
		_, d.err = d.w.WriteString(s)
	}
}

func (d *driver) outByte(c byte) {
	if d.err == nil {
		d.err = d.w.WriteByte(c)
	}
}

// flush empties the buffer and reports whether output has failed.
func (d *driver) flush() bool {
	if err := d.w.Flush(); err != nil && d.err == nil {
		d.err = err
	}
	return d.err == nil
}

func newDriver(db *softmagic.Database, o options, stdin, stdout *os.File) *driver {
	limit := o.maxBytes
	if limit <= 0 {
		limit = softmagic.DefaultMaxBytes
	}
	return &driver{db: db, o: o, stdin: stdin, w: bufio.NewWriterSize(stdout, 64*1024), buf: make([]byte, limit)}
}

// processNames is the reference's loop: one width for the whole list,
// except that -n makes a -f list print each name at its own width.
func (d *driver) processNames(names []string, fromList bool) bool {
	wid := maxWidth(names, d.o.raw)
	ok := true
	for _, name := range names {
		if fromList && d.o.noBuffer {
			wid = nameWidth(name, d.o.raw)
		}
		if !d.process(name, wid) {
			ok = false
		}
	}
	return ok
}

// process is file.c's process(): one line for one name. It reports
// whether the line was error-free.
func (d *driver) process(name string, wid int) bool {
	if wid > 0 && !d.o.brief {
		d.printName(name, wid)
	}
	text, ok := d.answer(name)
	d.out(text)
	if d.o.nulSep > 1 {
		d.outByte(0)
	} else {
		d.outByte('\n')
	}
	if d.o.noBuffer {
		return d.flush() && ok
	}
	return ok && d.err == nil
}

// printName is the "name:   " part, with file(1)'s padding and -0 forms.
func (d *driver) printName(name string, wid int) {
	pname := name
	if name == "-" {
		pname = "/dev/stdin"
	}
	d.out(nameText(pname, d.o.raw))
	if d.o.nulSep > 0 {
		d.outByte(0)
	}
	if d.o.nulSep < 2 {
		d.out(d.o.separator)
		pad := 0
		if !d.o.noPad {
			pad = wid - nameWidth(name, d.o.raw)
		}
		d.out(strings.Repeat(" ", pad))
		d.outByte(' ')
	}
}

// answer is magic_file: the stat layer, then the file's bytes. The
// reference prints "ERROR: (null)" when nothing at all was said.
func (d *driver) answer(name string) (string, bool) {
	if name == "-" {
		return d.finish(d.fromFile(d.stdin, "", ""))
	}
	text, st := fsmagic(d.o, name)
	switch st {
	case fsError:
		return "ERROR: " + text, false
	case fsDone:
		return d.finish(text, true)
	}
	f, err := openForRead(name)
	if err != nil {
		if info, serr := os.Stat(name); serr == nil {
			text += unreadableInfo(name, info.Mode())
		}
		return d.finish(text, true)
	}
	defer func() { _ = f.Close() }()
	return d.finish(d.fromFile(f, name, text))
}

// finish is file_getbuffer: nothing said is an error with no message,
// and an answer has its unprintable characters escaped unless -r.
func (d *driver) finish(text string, ok bool) (string, bool) {
	if ok && text == "" {
		return "ERROR: (null)", false
	}
	if ok && !d.o.raw {
		return escapeOutput(text), ok
	}
	return text, ok
}

// escapeOutput is file_getbuffer's escaping: valid UTF-8 keeps its
// printable characters and escapes the bytes of the others; anything
// else is escaped byte by byte as the C locale would.
func escapeOutput(text string) string {
	if !utf8.ValidString(text) {
		var b strings.Builder
		for i := 0; i < len(text); i++ {
			if c := text[i]; c >= 0x20 && c < 0x7f {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, "\\%03o", c)
			}
		}
		return b.String()
	}
	clean := true
	for _, r := range text {
		if !unicode.IsPrint(r) && r != ' ' {
			clean = false
			break
		}
	}
	if clean {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsPrint(r) || r == ' ' {
			b.WriteString(text[i : i+size])
		} else {
			for k := 0; k < size; k++ {
				fmt.Fprintf(&b, "\\%03o", text[i+k])
			}
		}
		i += size
	}
	return b.String()
}

// fromFile is file_or_fd after the open: read the window and identify
// it. A pipe is read until a short read; anything else with one read.
func (d *driver) fromFile(f *os.File, name, prefix string) (string, bool) {
	info, serr := f.Stat()
	okstat := serr == nil
	opts := softmagic.Options{MaxBytes: d.o.maxBytes, Continue: d.o.keepGoing, Raw: d.o.raw}
	if okstat {
		opts.Executable = info.Mode()&0o111 != 0
	}
	if okstat && info.Mode()&os.ModeNamedPipe != 0 {
		n := readPipe(f, d.buf)
		if n == 0 && name != "" {
			return prefix + unreadableInfo(name, info.Mode()), true
		}
		r := d.db.IdentifyWith(context.Background(), d.buf[:n], opts)
		return d.render(name, prefix, r), true
	}
	if okstat && info.Mode().IsRegular() && name != "" {
		r := d.db.IdentifyAt(context.Background(), f, info.Size(), opts)
		return d.render(name, prefix, r), true
	}
	n, err := readOnce(f, d.buf)
	if err != nil && err != io.EOF {
		if name == "" {
			name = "/dev/stdin"
		}
		return "ERROR: " + prefix + "cannot read `" + name + "' (" + cerror(err) + ")", false
	}
	r := d.db.IdentifyWith(context.Background(), d.buf[:n], opts)
	return d.render(name, prefix, r), true
}

// readPipe is the reference's pipe loop: read until the buffer is full
// or a read returns fewer than PIPE_BUF bytes.
func readPipe(f *os.File, buf []byte) int {
	n := 0
	for n < len(buf) {
		r, err := readOnce(f, buf[n:])
		if r > 0 {
			n += r
		}
		if err != nil || r < pipeBuf {
			break
		}
	}
	return n
}

// render formats a Result in the output mode after the stat layer's
// prefix. The reference's text phase sees the prefix as text already
// printed and puts ", " before its own description unless a text rule
// printed something; that quirk is reproduced.
func (d *driver) render(name, prefix string, r softmagic.Result) string {
	if d.o.keepGoing && d.o.mode != modeJSON {
		return d.renderContinued(prefix, r)
	}
	switch d.o.mode {
	case modeMime:
		return r.MIME + "; charset=" + r.Charset
	case modeMimeType:
		return r.MIME
	case modeMimeEncoding:
		return r.Charset
	case modeExtension:
		if len(r.Extensions) == 0 {
			return "???"
		}
		return strings.Join(r.Extensions, "/")
	case modeApple:
		if r.Apple == "" {
			return "UNKNUNKN"
		}
		return r.Apple
	case modeJSON:
		return renderJSON(name, prefix, r)
	default:
		if prefix != "" && r.Phase == softmagic.PhaseText && len(r.Rules) == 0 {
			return prefix + ", " + r.Description
		}
		return prefix + r.Description
	}
}

// renderContinued is render under -k: each mode's list joined with the
// reference's separator, which the output escaping later shows as \012-
// unless -r is given.
func (d *driver) renderContinued(prefix string, r softmagic.Result) string {
	c := r.Continued
	switch d.o.mode {
	case modeMime:
		return joinAnswers(c.MIMEs) + "; charset=" + r.Charset
	case modeMimeType:
		return joinAnswers(c.MIMEs)
	case modeMimeEncoding:
		return joinAnswers(c.Encodings)
	case modeExtension:
		return joinAnswers(c.Extensions)
	case modeApple:
		return joinAnswers(c.Apple)
	default:
		// As without -k: when the text phase answered first with no rule,
		// nothing preceded it, and the stat layer's words are joined to it.
		if prefix != "" && r.Phase == softmagic.PhaseText && len(r.Rules) == 0 {
			return prefix + ", " + joinAnswers(c.Descriptions)
		}
		return prefix + joinAnswers(c.Descriptions)
	}
}

// joinAnswers is the reference's -k output: answers separated by
// FILE_SEPARATOR.
func joinAnswers(a []string) string { return strings.Join(a, "\n- ") }

// jsonLine is the --json record: the name, the stat layer's mode words
// and the library's Result.
type jsonLine struct {
	Name   string           `json:"name"`
	Modes  string           `json:"modes,omitempty"`
	Result softmagic.Result `json:"result"`
}

func renderJSON(name, prefix string, r softmagic.Result) string {
	if name == "" {
		name = "/dev/stdin"
	}
	b, err := json.Marshal(jsonLine{Name: name, Modes: strings.TrimSpace(prefix), Result: r})
	if err != nil {
		return `{"error":"` + err.Error() + `"}`
	}
	return string(b)
}
