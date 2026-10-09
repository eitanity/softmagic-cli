// Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
// Use of this source code is governed by the BSD-2-Clause licence in LICENSE.

//go:build unix

package main

import (
	"os"
	"strings"
	"testing"
)

// TestSafeTextOutput: --safe-text escapes text from files, names, link
// targets and the names in the stat layer's messages as \xHH, outside
// printable ASCII and for \ < > & " ' and `, including under -r; -k's
// separator and --json's name follow it.
func TestSafeTextOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, content := range map[string]string{
		"a<b>":        "#!/usr/bin/env <img src=x onerror=alert(1)>\n",
		"caf\xc3\xa9": "#!/usr/bin/env py\\012thon\n",
		"e\x1bsc":     "#!/usr/bin/env x\x1b[31m\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("t\"arget", "l'ink"); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{ // padded to the longest name as it is printed
		"a\\x3cb\\x3e:  a \\x3cimg src=x onerror=alert(1)\\x3e script, ASCII text executable",
		"caf\\xc3\\xa9: a py\\x5c012thon script, ASCII text executable",
		"e\\x1bsc:     a x\\x1b[31m script, ASCII text executable, with escape sequences",
		"l\\x27ink:    broken symbolic link to t\\x22arget",
		"no\\x26such:  cannot open `no\\x26such' (No such file or directory)",
		"",
	}, "\n")
	args := []string{"a<b>", "caf\xc3\xa9", "e\x1bsc", "l'ink", "no&such"}
	for _, extra := range [][]string{{"--safe-text"}, {"--safe-text", "-r"}, {"-r", "--safe-text"}} {
		out, _, _ := capture(t, "", append(extra, args...)...)
		if out != want {
			t.Errorf("%v:\n got %q\nwant %q", extra, out, want)
		}
	}
	out, _, _ := capture(t, "", "--safe-text", "-b", "-k", "a<b>")
	if want := "a \\x3cimg src=x onerror=alert(1)\\x3e script text executable\\x0a- " +
		"a /usr/bin/env \\x3cimg src=x onerror=alert(1)\\x3e script, ASCII text executable\n"; out != want {
		t.Errorf("-k:\n got %q\nwant %q", out, want)
	}
	out, _, _ = capture(t, "", "--safe-text", "--json", "a<b>")
	if !strings.HasPrefix(out, `{"name":"a\\x3cb\\x3e",`) || !strings.Contains(out, `a \\x3cimg src=x onerror=alert(1)\\x3e script`) {
		t.Errorf("--json: %s", out)
	}
	out, _, _ = capture(t, "", "a<b>")
	if out != "a<b>: a <img src=x onerror=alert(1)> script, ASCII text executable\n" {
		t.Errorf("without --safe-text the output must be file's: %q", out)
	}
}
