// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The reference prints file names through the C locale's wide-character
// functions: a printable character as itself, anything else as a
// three-digit octal escape that counts four columns. This assumes the
// UTF-8 locale every modern system runs; East Asian wide characters are
// counted as one column where the reference counts two.

// nameText is fname_print: the name as it is printed.
func nameText(name string, raw bool) string {
	if raw {
		return name
	}
	var b strings.Builder
	for i := 0; i < len(name); {
		r, size := utf8.DecodeRuneInString(name[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "\\%03o", name[i])
		case unicode.IsPrint(r):
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
	w := 0
	for i := 0; i < len(name); {
		r, size := utf8.DecodeRuneInString(name[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			w += 4
		case raw || unicode.IsPrint(r):
			w++
		default:
			w += 4
		}
		i += size
	}
	return w
}

// low8 is the low byte of a rune, as the reference's cast takes it.
func low8(r rune) byte { return byte(r & 0xff) }

// maxWidth is the padding width for a list of names.
func maxWidth(names []string, raw bool) int {
	wid := 0
	for _, n := range names {
		if w := nameWidth(n, raw); w > wid {
			wid = w
		}
	}
	return wid
}
