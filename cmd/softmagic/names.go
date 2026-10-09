// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// The reference prints file names through the C library's wide-character
// functions: a character iswprint accepts as itself, counting wcwidth's
// columns (one when that is zero), anything else as a three-digit octal
// escape that counts four. This assumes the UTF-8 locale every modern
// system runs, and takes iswprint and wcwidth from glibc's tables for it
// (wctype.go), which accept more than Go's unicode.IsPrint: private-use
// characters, non-ASCII spaces and format characters such as U+202E.

// printRange is a run of code points iswprint accepts, all of one width.
type printRange struct {
	lo, hi rune
	width  int
}

// printWidth is the columns the reference counts for a printable r, or 0
// when iswprint rejects it and r is escaped.
func printWidth(r rune) int {
	lo, hi := 0, len(printRanges)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		switch p := printRanges[mid]; {
		case r < p.lo:
			hi = mid
		case r > p.hi:
			lo = mid + 1
		default:
			return p.width
		}
	}
	return 0
}

// nameText is fname_print: the name as it is printed.
func nameText(name string, raw bool) string {
	if raw {
		return name
	}
	var b strings.Builder        // escaped
	for i := 0; i < len(name); { // byteIndex
		r, size := utf8.DecodeRuneInString(name[i:]) // char
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "\\%03o", name[i])
		case printWidth(r) > 0:
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\%03o", low8(r))
		}
		i += size
	}
	return b.String()
}

// nameWidth is file_mbswidth: the columns the printed name occupies.
func nameWidth(name string, raw bool) int {
	w := 0                       // width
	for i := 0; i < len(name); { // byteIndex
		r, size := utf8.DecodeRuneInString(name[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			w += 4
		case printWidth(r) > 0:
			w += printWidth(r)
		case raw:
			w++ // wcwidth is -1 for what iswprint rejects; file counts it as one
		default:
			w += 4
		}
		i += size
	}
	return w
}

// low8 is the low byte of a rune, as the reference's cast takes it.
func low8(r rune) byte { return byte(r & 0xff) }

// safeName is a name, a link target or text from a file under --safe-text:
// printable ASCII other than \ < > & " ' and ` as itself, every other byte
// as \xHH, so it can neither open markup nor end a quoted value, and each
// backslash starts an escape. It counts one column per byte.
func safeName(name string) string {
	var b strings.Builder // escaped
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c < 0x20 || c > 0x7e, c == '\\', c == '<', c == '>', c == '&', c == '"', c == '\'', c == '`':
			fmt.Fprintf(&b, "\\x%02x", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// safeOutput is the last step of a --safe-text answer: the library has
// escaped what came from the file and the names are escaped already, so
// only this program's own control characters remain, such as the newline
// of the -k separator, and each becomes \xHH.
func safeOutput(text string) string {
	var b strings.Builder // escaped
	for i := 0; i < len(text); i++ {
		if c := text[i]; c < 0x20 || c > 0x7e {
			fmt.Fprintf(&b, "\\x%02x", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// shownName is the name as this run prints it.
func (o options) shownName(name string) string {
	if o.safeText {
		return safeName(name)
	}
	return nameText(name, o.raw)
}

// shownWidth is the columns shownName occupies.
func (o options) shownWidth(name string) int {
	if o.safeText {
		return len(safeName(name))
	}
	return nameWidth(name, o.raw)
}

// fsText is a name or link target inside one of the stat layer's answers
// or errors: as it is, as file prints it, or escaped under --safe-text.
func (o options) fsText(name string) string {
	if o.safeText {
		return safeName(name)
	}
	return name
}

// maxWidth is the padding width for a list of names.
func maxWidth(names []string, o options) int {
	wid := 0
	for _, n := range names {
		if w := o.shownWidth(n); w > wid {
			wid = w
		}
	}
	return wid
}
