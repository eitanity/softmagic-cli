# softmagic-cli

`softmagic` is `file(1)` over the [softmagic](https://github.com/eitanity/softmagic)
library: a pure-Go port of file 5.48 that prints what `file` prints, byte for
byte, for every mode the library implements, with the same exit codes.

## Install

```
go install github.com/eitanity/softmagic-cli/cmd/softmagic@latest
```

The binary is called `softmagic`. It is built against the tagged library,
`github.com/eitanity/softmagic` v0.1.0.

## Usage

```
softmagic [-bEhiLnNrs0] [--apple] [--extension] [--mime-encoding] [--mime-type]
          [--json] [-F <separator>] [-m <magicdirs>] [--extra-magic <dirs>]
          [--max-bytes <n>] [-P bytes=<n>] [-f <namefile>] <file> ...
softmagic --version
```

| Flag | Meaning, as in `file(1)` |
|---|---|
| `-b` | brief: no file name before the answer |
| `-i`, `--mime` | MIME type and charset (`text/plain; charset=us-ascii`) |
| `--mime-type`, `--mime-encoding` | one half of `-i` |
| `--extension` | the rule's extension list, `???` when none |
| `--apple` | the rule's Apple creator/type, `UNKNUNKN` when none |
| `--json` | one JSON object per line: the name, the stat layer's mode words and the library's `Result`, so what was examined is visible |
| `-N` | no padding of the answers into a column |
| `-n` | flush after every file; a `-f` list prints each name at its own width |
| `-0` | a NUL after the file name; `-00` and no separator or newline at all |
| `-F sep` | the separator after the name, default `:` |
| `-E` | a file that cannot be stat'ed is an error (`ERROR: ...`, exit 1) rather than an answer |
| `-h` / `-L` | do not / do follow symbolic links (default: do not) |
| `-s` | read block and character devices, and empty regular files, instead of describing the inode |
| `-r` | raw: no octal escaping of unprintable characters in names and answers |
| `-f file` | file names one per line from `file`, `-` for standard input, printed before any names on the command line |
| `-m dirs` | replace the embedded database with the magic source in these colon-separated directories |
| `--extra-magic dirs` | append magic source directories to whichever database is in use |
| `--max-bytes n`, `-P bytes=n` | consult at most `n` bytes of each file (default 7 MiB, as `file`) |
| `-` | standard input, named `/dev/stdin` |

`-k` (keep going) and `-z` (look inside compressed files) are refused: the
first is not in the library and the second is a non-goal of it. The other `-P`
parameters are limits the library fixes.

## What is identical

Over the library's parity corpus and a set of special files, the output of
`softmagic` and of the reference `file -m Magdir` built from the 5.48 tarball
is byte-identical, exit code included, for every mode listed above and their
combinations with `-b`, `-N`, `-0`, `-00`, `-F`, `-n`, `-r`, `-s`, `-L`, `-E`,
`-f` and standard input. That covers:

- the stat layer from `fsmagic.c`: directories, symbolic links (followed only
  with `-L`, read through only in the `--extension` and `--apple` modes, as
  `file` does), dangling links, fifos, sockets, character and block specials
  with their device numbers, empty files, and the `setuid`, `setgid` and
  `sticky` prefix;
- the padding rule (`name:` then spaces to the longest name, then one space),
  the multibyte width of names, and the octal escaping of unprintable
  characters in names and answers (`\011`);
- files that cannot be opened or read (`writable, no read permission`,
  `ERROR: cannot read `d' (Is a directory)`);
- reading a pipe until the first short read, and a single read for everything
  else, as `file_or_fd` does;
- `file`'s own quirks, reproduced on purpose so that scripts see the same
  thing: `ERROR: (null)` with exit 1 for a dangling link under `--mime-encoding`
  and for a fifo under `-s`, and `setgid , ASCII text` (with the stray comma)
  for a text file carrying a mode bit, which is why the library's `Result`
  carries a `Phase`.

Known differences:

- `--version` and the usage text are this program's own.
- `-m` and `--extra-magic` take source directories, not compiled `.mgc` files.
- A name holding an East Asian wide character pads one column short of `file`.
- `file`'s `-C`, `-c`, `-d`, `-e`, `-l`, `-p`, `-S`, `-z`, `-Z`, `-k` and
  `--exclude-quiet` are not provided.

## Platforms

The library is pure Go with explicit byte orders everywhere, and vets for linux (386, arm,
arm64, ppc64le, s390x, mips, riscv64, amd64), darwin, freebsd, openbsd and windows; its suite
also runs natively as a 32-bit binary. This command is developed and compared against the
reference on linux/amd64. It builds for the same targets; the stat layer has a Unix form
(device numbers from `st_rdev` split as glibc does, `access(2)`, non-blocking opens) and a
Windows form (no device numbers, permission bits instead of `access`), of which only the Unix
form has been checked against `file`. 

## Verifying it yourself

`make test` runs the CLI over the library's corpus (`testdata/corpus` of
`github.com/eitanity/softmagic`, cloned beside this checkout as `../softmagic`; without it
those tests skip), four modes per file against expectations produced by the reference, and over a
temporary tree of special files whose expected answers were transcribed from the reference.

Those expectations are stored. To compare against the reference itself:

```
make compare DIRS=/usr/bin:/etc:/dev
```

`make reference` (which `compare` runs first) downloads file 5.48's release tarball, checks
it against a pinned SHA-256, and builds it statically into `.reference/`, with the database
compiled from that release's Magdir. `make compare` then runs `softmagic` and that binary side
by side over the corpus and every path under `DIRS`, in ten output modes (default, `-b`, `-i`,
`--mime-type`, `--mime-encoding`, `--extension`, `--apple`, `-L`, `-N`, `-E`). It requires
stdout, stderr and the exit code to be byte-identical, and lists every difference. Building the
reference needs a C compiler and `make`. A host's own `file` is not a substitute: it is usually
another release.

## Licence

BSD-2-Clause, Copyright (c) 2026 Eitanity Systems VCC; see `LICENSE`. The stat
layer follows `fsmagic.c` from file 5.48, whose notice is in `COPYING`.
