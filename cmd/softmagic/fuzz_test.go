// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"bufio"
	"bytes"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// FuzzSplitNames: -f's reading against a model of getline: lines split at
// newlines, a last line without one kept, nothing else removed, each name
// ending at a NUL.
func FuzzSplitNames(f *testing.F) {
	f.Add([]byte("a\n\nb\r\nc"))
	f.Add([]byte("x\x00y\n"))
	f.Add(bytes.Repeat([]byte("n"), 100000))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := splitNames(bufio.NewReader(bytes.NewReader(data)))
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		parts := bytes.Split(data, []byte("\n"))
		if len(parts[len(parts)-1]) == 0 {
			parts = parts[:len(parts)-1] // getline returns nothing after the last newline
		}
		for _, p := range parts {
			if nul := bytes.IndexByte(p, 0); nul >= 0 {
				p = p[:nul]
			}
			want = append(want, string(p))
		}
		if !slices.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// FuzzAtoi: -P's number reading against a model of glibc's atoi, which is
// strtol (blanks, a sign, digits, saturating at the long range) cast to int.
func FuzzAtoi(f *testing.F) {
	for _, s := range []string{"12", " -7x", "+", "99999999999", "999999999999999999999", "-9223372036854775809", "\t\n42"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, gotSat := atoi(s)
		want, wantSat := atoiModel(s)
		if got != want || gotSat != wantSat {
			t.Fatalf("atoi(%q) = %d %v, want %d %v", s, got, gotSat, want, wantSat)
		}
	})
}

// atoiModel computes atoi with big integers.
func atoiModel(s string) (int, bool) { // text
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := strings.HasPrefix(s, "-")
	if neg || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v := new(big.Int) // value
	if end > 0 {
		v.SetString(s[:end], 10)
	}
	if neg {
		v.Neg(v)
	}
	sat := false
	lo, hi := big.NewInt(-1<<63), big.NewInt(1<<63-1)
	switch {
	case v.Cmp(hi) > 0:
		v, sat = hi, true
	case v.Cmp(lo) < 0:
		v, sat = lo, true
	}
	low := v.Int64() & 0xffffffff // the int cast keeps the low 32 bits, signed
	if low >= 1<<31 {
		low -= 1 << 32
	}
	return int(low), sat
}

// FuzzGetopt: any argument vector parses without panicking, the same way
// twice, into names that were arguments and events that name known
// options, with or without POSIXLY_CORRECT.
func FuzzGetopt(f *testing.F) {
	for _, a := range []string{"-bik\x00x", "--mime-t\x00a\x00-P\x00bytes=1", "--ex\x00json", "a\x00--\x00-b", "-e", "--=x\x00-"} {
		f.Add(a, false)
		f.Add(a, true)
	}
	f.Fuzz(func(t *testing.T, joined string, posixly bool) {
		args := strings.Split(joined, "\x00")
		p1, p2 := parsePlan(args, posixly), parsePlan(args, posixly)
		if !reflect.DeepEqual(p1, p2) {
			t.Fatalf("two parses of %q differ", args)
		}
		events, names, _ := getoptLong(args, posixly)
		for _, n := range names {
			if !slices.Contains(args, n) {
				t.Fatalf("name %q was not an argument of %q", n, args)
			}
		}
		for _, ev := range events {
			if ev.spec == nil || !slices.ContainsFunc(optSpecs, func(s optSpec) bool { return s == *ev.spec }) {
				t.Fatalf("event with an unknown option from %q", args)
			}
		}
	})
}
