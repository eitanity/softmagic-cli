# softmagic-cli

`softmagic` is `file(1)` over the [softmagic](https://github.com/eitanity/softmagic)
library: a pure-Go port of file 5.48 that prints what `file` prints, byte for
byte, for every mode the library implements, with the same exit codes.

## Install

```
go install github.com/eitanity/softmagic-cli/cmd/softmagic@latest
```

The binary is called `softmagic`. It is built against the tagged library
`github.com/eitanity/softmagic`; `softmagic --version` names both versions.

## Usage

```
softmagic [-bEhikLnNprsS0] [--apple] [--extension] [--mime-encoding] [--mime-type]
          [--json] [-F <separator>] [-m <magicdirs>] [--extra-magic <dirs>]
          [--max-bytes <n>] [-P <name>=<value>] [-e <check>] [--exclude-quiet <check>]
          [-f <namefile>] <file> ...
softmagic [-m <magicdirs>] -l
softmagic [-m <magicdirs>] -c
softmagic --version
```

Options are read as `file`'s `getopt_long` reads them: every option has its long name too
(`--brief`, `--keep-going`, `--magic-file`, ...; `softmagic --help` lists them), a long name may
be cut to any unambiguous prefix (`--mime-t`), short options group (`-bik`) and take their value
attached (`-ejson`, `-Pbytes=10`, `-F@`), options may follow file names, `--` ends them, and
`POSIXLY_CORRECT` makes the first name end them and `-L` the default. A `-f` list is read where
the `-f` stands: options after the first `-f` change only what is printed (`-b`, `-N`, `-0`,
`-F`, `-n`) and the `-P` limits, as in `file`.

| Flag | Meaning, as in `file(1)` |
|---|---|
| `-b` | brief: no file name before the answer |
| `-i`, `--mime` | MIME type and charset (`text/plain; charset=us-ascii`) |
| `--mime-type`, `--mime-encoding` | one half of `-i` |
| `--extension` | the rule's extension list, `???` when none |
| `--apple` | the rule's Apple creator/type, `UNKNUNKN` when none |
| `--json` | one JSON object per line and nothing else: `name`, then the library's `result` (with the stat layer's words before it in `modes`), or the stat layer's own `answer` for a directory, device or link, or an `error` |
| `-N` | no padding of the answers into a column |
| `-n` | flush after every file; a `-f` list prints each name at its own width |
| `-0` | a NUL after the file name; `-00` and no separator or newline at all |
| `-F sep` | the separator after the name, default `:` |
| `-E` | a file that cannot be stat'ed is an error (`ERROR: ...`, exit 1) rather than an answer |
| `-h` / `-L` | do not / do follow symbolic links (default: do not) |
| `-s` | read block and character devices, and empty regular files, instead of describing the inode |
| `-r` | raw: no octal escaping of unprintable characters in names and answers |
| `-k` | keep going: every match, each after a `\012- ` separator (a newline with `-r`), in any of the output modes above |
| `-f file` | file names one per line from `file`, `-` for standard input, printed before any names on the command line |
| `-m dirs` | replace the embedded database with the magic source in these colon-separated directories |
| `--extra-magic dirs` | append magic source directories to whichever database is in use |
| `--max-bytes n` | consult at most `n` bytes of each file (default 7 MiB, as `file`) |
| `-P name=value` | one of `file`'s limits: `bytes`, `elf_notes`, `elf_phnum`, `elf_shnum`, `elf_shsize`, `encoding`, `indir`, `name`, `regex`, `magwarn` (accepted, no effect); repeatable, and the name may be cut short (`-P b=100`) as `file` allows. Reaching `indir`, `name` or `elf_shsize` is an error, `ERROR: ...` and exit 1, as in `file` |
| `-e check`, `--exclude-quiet check` | switch a check off: `apptype`, `ascii`, `cdf`, `compress`, `csv`, `elf`, `encoding`, `json`, `simh`, `soft`, `tar`, `text`, `tokens`; an unknown name is a usage error after `-e` and ignored after `--exclude-quiet` |
| `-l` | print the database in matching order, as `file -l` |
| `-c` | print the parsed form of every rule line in `-m`'s directories, on standard error, as `file -c` does; the built-in database is compiled already, as a `.mgc` file is, and prints nothing |
| `-p` | put back each file's access time after reading it |
| `-S` | accepted, no effect: there is no sandbox to switch off |
| `-` | standard input, named `/dev/stdin` |

`-z` and `-Z` (look inside compressed files), `-C` (write a `.mgc`) and `-d` (libmagic's debug
trace) are refused by name: they are non-goals of the library. `-c` reports a refused rule in
the library's words, not libmagic's.

## What is identical

Over the library's parity corpus and a set of special files, the output of
`softmagic` and of the reference `file -m Magdir` built from the 5.48 tarball
is byte-identical, exit code included, for every mode listed above and their
combinations with `-k`, `-b`, `-N`, `-0`, `-00`, `-F`, `-n`, `-r`, `-s`, `-L`, `-E`,
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

- `-m` and `--extra-magic` take source directories, not compiled `.mgc` files.
- A name holding an East Asian wide character pads one column short of `file`.
- `file`'s `-C`, `-d`, `-z` and `-Z` are not provided. `-c` reports a refused rule in the
  library's words, after the lines before it, where `file` warns and goes on. With several
  `-m` directories, `file -c` prefixes the second and later warnings with bytes read from freed
  memory; this program prints those warnings without a prefix.
- `--help`, `--version` and the usage text are this program's own, as are the words of an
  option error; the exit codes are `file`'s.
- Beyond any default limit: where `file` reports "Output buffer space exceeded" (one printed
  piece over 1,024 bytes), the library cuts the answer at 4,096 bytes instead; `use` nesting
  stops at 32 levels before a `-P name` above 31 is reached; and an ELF note section over
  16 MiB is not read, where `file` reads up to `elf_shsize` (128 MiB by default). On a file
  larger than the window, a `search`, `regex`, `der`, `use` or `indirect` rule counted from the
  end still counts from the window's end; rules that read one value read the file's own tail,
  as `file` does.
- An error message about `-P` or `-e` names this program, not `file`.

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
by side over the corpus and every path under `DIRS`, in 63 modes: default, `-b`, `-i`,
`--mime-type`, `--mime-encoding`, `--extension`, `--apple`, `-L`, `-N`, `-E`, `-r`; `-k` with
each output mode, and `-k -r -E` with and without `--mime-type` (the flags azul passes to
libmagic); every `-e` name, alone and with `-i`, `-k` and `--extension`; and every `-P`
parameter at a value that changes answers, including the three that stop with an error, alone
and with the other modes. Each mode is a subtest, so `go test -run 'TestReference/-k'` selects
some. It requires stdout, stderr and the exit code to be byte-identical, and lists every difference. Building the
reference needs a C compiler and `make`. A host's own `file` is not a substitute: it is usually
another release.

## Licence

BSD-2-Clause, Copyright (c) 2026 Eitanity Systems VCC; see `LICENSE`. The stat
layer follows `fsmagic.c` from file 5.48, whose notice is in `COPYING`.
