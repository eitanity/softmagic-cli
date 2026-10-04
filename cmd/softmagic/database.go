// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/eitanity/softmagic"
	"github.com/eitanity/softmagic/compile"
)

const version = "0.1.0"

// loadDatabase is file(1)'s magic selection: -m replaces the default
// database with the colon-separated directories, each its own sorted map
// searched in order; --extra-magic appends directories to whichever
// database is in use.
func loadDatabase(o options) (*softmagic.Database, error) {
	var db *softmagic.Database
	var err error
	if o.magicDirs == "" {
		if db, err = softmagic.Default(); err != nil {
			return nil, err
		}
	} else {
		if db, err = compileDirs(nil, o.magicDirs); err != nil {
			return nil, err
		}
	}
	if o.extraDirs != "" {
		return compileDirs(db, o.extraDirs)
	}
	return db, nil
}

// compileDirs compiles each directory of a colon-separated list and
// appends it, resolving "use" names against what came before.
func compileDirs(base *softmagic.Database, dirs string) (*softmagic.Database, error) {
	for _, dir := range strings.Split(dirs, ":") {
		if dir == "" {
			continue
		}
		extra, err := compile.Compile(os.DirFS(dir), softmagic.CompileOptions{SourceDir: dir, Base: base})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		if base == nil {
			base = extra
			continue
		}
		if base, err = compile.Append(base, extra); err != nil {
			return nil, err
		}
	}
	if base == nil {
		return nil, fmt.Errorf("no magic directories in `%s'", dirs)
	}
	return base, nil
}

func printVersion(w io.Writer) {
	hash := "unavailable"
	if db, err := softmagic.Default(); err == nil {
		hash = db.Hash()
	}
	_, _ = fmt.Fprintf(w, "softmagic-cli %s\nimplements file %s\nmagic database %s\n", version, softmagic.ImplementsFile, hash)
}
