// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.
//
// The stat layer, after fsmagic.c in file 5.48, Copyright (c) Ian F.
// Darwin 1986-1995 (see COPYING): what file(1) says about a name before
// it reads any bytes. Its quirks are reproduced on purpose, including the
// cases where the reference prints nothing and the driver then reports
// "ERROR: (null)"; the README lists them.

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// The device-number, access-check and non-blocking-open details differ
// per platform; see fsmagic_unix.go and fsmagic_windows.go.

const (
	fsError    = -1 // text is an error message
	fsContinue = 0  // text is a prefix; go on to read the file
	fsDone     = 1  // text is the answer
)

// fsBuf is the reference's output buffer with its COMMA counter.
type fsBuf struct {
	text string
	did  int
}

// add is file_printf("%s...", COMMA): a comma before every item but the
// first.
func (b *fsBuf) add(s string) {
	if b.did > 0 {
		b.text += ", "
	}
	b.did++
	b.text += s
}

// mime is handle_mime: an inode type in the MIME modes.
func (b *fsBuf) mime(o options, what string) {
	switch o.mode {
	case modeMime:
		b.text += "inode/" + what + "; charset=binary"
	case modeMimeType:
		b.text += "inode/" + what
	default:
		b.text += "binary"
	}
}

func isMime(o options) bool {
	return o.mode == modeMime || o.mode == modeMimeType || o.mode == modeMimeEncoding
}

func isSilent(o options) bool { return o.mode == modeExtension || o.mode == modeApple }

// fsmagic is file_fsmagic for one name: the text so far and whether the
// caller goes on to read the file.
func fsmagic(o options, name string) (string, int) {
	var info os.FileInfo
	var err error
	if o.followLinks {
		info, err = os.Stat(name)
	} else {
		info, err = os.Lstat(name)
	}
	if err != nil {
		if o.errExit {
			return fmt.Sprintf("cannot stat `%s' (%s)", name, cerror(err)), fsError
		}
		return fmt.Sprintf("cannot open `%s' (%s)", name, cerror(err)), fsContinue
	}
	var b fsBuf
	mime, silent := isMime(o), isSilent(o)
	if !mime && !silent {
		modePrefix(&b, info.Mode())
	}
	ret := typeAnswer(o, name, info, &b)
	if ret == fsError {
		return b.text, ret
	}
	if !silent && !mime && b.did > 0 && ret == fsContinue {
		b.text += " "
	}
	if ret == fsDone && silent {
		// Not the silent modes' job to print here, so not a match.
		return b.text, fsContinue
	}
	return b.text, ret
}

// modePrefix is the "setuid", "setgid", "sticky" list.
func modePrefix(b *fsBuf, m fs.FileMode) {
	if m&fs.ModeSetuid != 0 {
		b.add("setuid")
	}
	if m&fs.ModeSetgid != 0 {
		b.add("setgid")
	}
	if m&fs.ModeSticky != 0 {
		b.add("sticky")
	}
}

// inode records one inode type in the mode's wording.
func inode(o options, b *fsBuf, desc, what string) int {
	switch {
	case isMime(o):
		b.mime(o, what)
	case isSilent(o):
	default:
		b.add(desc)
	}
	return fsDone
}

// typeAnswer is the switch on the file type.
func typeAnswer(o options, name string, info os.FileInfo, b *fsBuf) int {
	m := info.Mode()
	switch {
	case m.IsDir():
		return inode(o, b, "directory", "directory")
	case m&fs.ModeCharDevice != 0:
		if o.devices {
			return fsContinue
		}
		return inode(o, b, "character special"+deviceNumbers(info), "chardevice")
	case m&fs.ModeDevice != 0:
		if o.devices {
			return fsContinue
		}
		return inode(o, b, "block special"+deviceNumbers(info), "blockdevice")
	case m&fs.ModeNamedPipe != 0:
		if o.devices {
			return fsDone // the reference's break leaves ret at 1 with nothing printed
		}
		return inode(o, b, "fifo (named pipe)", "fifo")
	case m&fs.ModeSocket != 0:
		return inode(o, b, "socket", "socket")
	case m&fs.ModeSymlink != 0:
		return symlinkAnswer(o, name, b)
	case m.IsRegular():
		if !o.devices && info.Size() == 0 {
			return inode(o, b, "empty", "x-empty")
		}
		return fsContinue
	default:
		b.text = fmt.Sprintf("invalid mode 0%o", uint32(m))
		return fsError
	}
}

// symlinkAnswer reports a symbolic link; the file is read through it
// only in the silent modes, as the reference does.
func symlinkAnswer(o options, name string, b *fsBuf) int {
	target, err := os.Readlink(name)
	if err != nil {
		if o.errExit {
			b.text = fmt.Sprintf("unreadable symlink `%s' (%s)", name, cerror(err))
			return fsError
		}
		return inode(o, b, fmt.Sprintf("unreadable symlink `%s' (%s)", name, cerror(err)), "symlink")
	}
	if _, err := os.Stat(name); err != nil {
		return badLink(o, b, target, err)
	}
	return inode(o, b, "symbolic link to "+target, "symlink")
}

// badLink is bad_link: a dangling symbolic link. Only the MIME type is
// printed in the MIME modes, so --mime-encoding prints nothing.
func badLink(o options, b *fsBuf, target string, err error) int {
	switch {
	case o.mode == modeMime || o.mode == modeMimeType:
		b.text += "inode/symlink"
	case isMime(o):
	case o.errExit:
		b.text = fmt.Sprintf("broken symbolic link to %s (%s)", target, cerror(err))
		return fsError
	default:
		b.text += "broken symbolic link to " + target
	}
	return fsDone
}

// cerror is C's strerror text for a Go error: the same message with its
// first letter in upper case.
func cerror(err error) string {
	var errno syscall.Errno
	msg := err.Error()
	if errors.As(err, &errno) {
		msg = errno.Error()
	}
	if msg == "" {
		return msg
	}
	b := []byte(msg)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
