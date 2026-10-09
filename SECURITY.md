# Security

## Reporting a vulnerability

Report privately through GitHub: the repository's **Security** tab, **Report a vulnerability**.
Please do not open a public issue for a vulnerability. Include the command line, the file or
directory that triggers it (or how to build it), and the output of `softmagic --version`.

A problem in identification itself (a crash, a hang or a wrong answer for some bytes) belongs to
the library: report it at
[github.com/eitanity/softmagic](https://github.com/eitanity/softmagic/security). Its
`SECURITY.md` describes what the library trusts and how to use it on untrusted input.

## Supported versions

softmagic-cli is pre-1.0. Fixes go into the latest release only; there are no backports.

| Version | Supported |
|---------|-----------|
| 0.3.x (latest) | yes |
| older   | no        |

## What softmagic trusts

| Input | Trust |
|-------|-------|
| File contents, file names, symbolic links and special files | untrusted |
| The command line, `-f` lists and `-P` values | from the user running it |
| Rule directories given to `-m` and `--extra-magic` | trusted, like code: rules choose every answer |

softmagic starts no shell or other process, uses no network, and writes no files. The only
environment variable it reads is `POSIXLY_CORRECT`; it ignores `MAGIC`. `-z`, `-Z`, `-C` and
`-d` are refused, so it never decompresses input and never writes a compiled database.

## Running softmagic over untrusted files

- **Use `--safe-text` when the output goes into a web page, a log or a quoted value.** Every
  byte taken from a file, a name or a link target is then written as `\xHH` unless it is
  printable ASCII other than `\ < > & " '` and the backquote, so nothing from the file can
  open markup, end a quoted value, move the terminal or hide behind a bidi control. The answer
  is otherwise the same, but it is no longer byte-for-byte `file`'s.
- **Do not use `-r` on untrusted files.** By default, unprintable bytes in file names and
  answers are written as `\ooo` escapes, as `file` writes them. `-r` turns that off, and escape
  sequences in a file's name or contents then reach your terminal.
- **Printable Unicode in names is printed as it is.** As `file` does in a UTF-8 locale, a name's
  characters are escaped only when glibc's `iswprint` rejects them. That accepts bidi controls
  such as U+202E, which make a name display reversed (`invoice\u202eFDP.exe` shows as
  `invoiceexe.PDF`), and zero-width characters. Do not judge a file by how its name looks on
  screen; `--safe-text` escapes them.
- **Parse the output with `-0` or `--json`.** File names may contain the separator, newlines
  or text that looks like an answer. `-0` ends each name with a NUL; `--json` writes one JSON
  object per file.
- **Leave `-s` off unless you need it.** Without `-s`, devices, fifos and sockets are described
  from `stat` alone and never read. With `-s` they are read: a fifo can stall the run, and a
  device's contents can appear in the answer.
- **`-L` follows symbolic links.** Without it, links are reported, not followed, except in the
  `--extension` and `--apple` modes, which read through them as `file` does.
- **`-p` restores times by name.** It puts back each file's access time after reading it, by
  path, as `file` does with `utimes`. If the path is replaced by a symbolic link in between,
  the times of the link's target are set instead.
- **The answer is not an allow-list.** softmagic gives the answer `file` 5.48 gives, from the
  first 7 MiB by default. A file can be built to be two things at once. Confirm the type before
  trusting the content.

## How the code is checked

`go test -race ./...` runs the option, stat-layer and output tests, a descriptor-leak test on
Linux, and fuzzers for option parsing (`FuzzGetopt`), `-P` numbers (`FuzzAtoi`) and `-f` lists
(`FuzzSplitNames`). The reference comparison runs every corpus file through `softmagic` and
`file` 5.48 in each output mode and requires identical output and exit codes.
