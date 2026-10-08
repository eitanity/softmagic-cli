// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/eitanity/softmagic"
	"github.com/eitanity/softmagic/compile"
)

// loadDatabase is file(1)'s magic selection: -m replaces the default
// database with the colon-separated directories, each its own sorted map
// searched in order; --extra-magic appends directories to whichever
// database is in use.
func loadDatabase(o options) (*softmagic.Database, error) { // opts
	var db *softmagic.Database // database
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

func printVersion(w io.Writer) { // out
	hash := "unavailable"
	if db, err := softmagic.Default(); err == nil {
		hash = db.Hash()
	}
	cli, lib := buildVersions()
	_, _ = fmt.Fprintf(w, "softmagic-cli %s\nlibrary %s\nimplements file %s\nmagic database %s\n",
		cli, lib, softmagic.ImplementsFile, hash)
}

// buildVersions are this module's version and the softmagic library's, as
// the go command recorded them in the binary: the tag for go install
// …@v0.1.2, a pseudo-version for an untagged commit, the checkout's VCS
// version (with +dirty for local changes) for go build, "(devel)" when
// none is known. Nothing here is kept in step with a tag by hand.
func buildVersions() (cli, lib string) {
	cli, lib = "(unknown)", "(unknown)"
	bi, ok := debug.ReadBuildInfo() // buildInfo
	if !ok {
		return cli, lib
	}
	if bi.Main.Version != "" {
		cli = bi.Main.Version
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/eitanity/softmagic" {
			lib = dep.Version
			if dep.Replace != nil {
				lib += " => " + dep.Replace.Path + " " + dep.Replace.Version
			}
		}
	}
	return cli, lib
}

// splitDirs is a colon-separated directory list, none when empty.
func splitDirs(dirs string) []string {
	if dirs == "" {
		return nil
	}
	return strings.Split(dirs, ":")
}
