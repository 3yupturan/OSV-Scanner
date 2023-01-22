// main cannot be accessed directly, so cannot use main_test
package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func dedent(t *testing.T, str string) string {
	t.Helper()

	// 0. replace all tabs with spaces
	str = strings.ReplaceAll(str, "\t", "  ")

	// 1. remove trailing whitespace
	re := regexp.MustCompile(`\r?\n([\t ]*)$`)
	str = re.ReplaceAllString(str, "")

	// 2. if any of the lines are not indented, return as we're already dedent-ed
	re = regexp.MustCompile(`(^|\r?\n)[^\t \n]`)
	if re.MatchString(str) {
		return str
	}

	// 3. find all line breaks to determine the highest common indentation level
	re = regexp.MustCompile(`\n[\t ]+`)
	matches := re.FindAllString(str, -1)

	// 4. remove the common indentation from all strings
	if matches != nil {
		size := len(matches[0]) - 1

		for _, match := range matches {
			if len(match)-1 < size {
				size = len(match) - 1
			}
		}

		re := regexp.MustCompile(`\n[\t ]{` + fmt.Sprint(size) + `}`)
		str = re.ReplaceAllString(str, "\n")
	}

	// 5. Remove leading whitespace.
	re = regexp.MustCompile(`^\r?\n`)
	str = re.ReplaceAllString(str, "")

	return str
}

// checks if two strings are equal, treating any occurrences of `%%` in the
// expected string to mean "any text"
func areEqual(t *testing.T, actual, expect string) bool {
	t.Helper()

	expect = regexp.QuoteMeta(expect)
	expect = strings.ReplaceAll(expect, "%%", ".+")

	re := regexp.MustCompile(`^` + expect + `$`)

	return re.MatchString(actual)
}

type cliTestCase struct {
	name         string
	args         []string
	wantExitCode int
	wantStdout   string
	wantStderr   string
}

func testCli(t *testing.T, tc cliTestCase) {
	t.Helper()

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}

	ec := run(tc.args, stdoutBuffer, stderrBuffer)
	// ec := run(tc.args, os.Stdout, os.Stderr)

	stdout := stdoutBuffer.String()
	stderr := stderrBuffer.String()

	if ec != tc.wantExitCode {
		t.Errorf("cli exited with code %d, not %d", ec, tc.wantExitCode)
	}

	if !areEqual(t, dedent(t, stdout), dedent(t, tc.wantStdout)) {
		t.Errorf("stdout\n got:\n%s\n\n want:\n%s", dedent(t, stdout), dedent(t, tc.wantStdout))
	}

	if !areEqual(t, dedent(t, stderr), dedent(t, tc.wantStderr)) {
		t.Errorf("stderr\n got:\n%s\n\n want:\n%s", dedent(t, stderr), dedent(t, tc.wantStderr))
	}
}

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []cliTestCase{
		{
			name:         "",
			args:         []string{""},
			wantExitCode: 128,
			wantStdout:   "",
			wantStderr: `
        No package sources found, --help for usage information.
			`,
		},
		{
			name:         "",
			args:         []string{"", "--version"},
			wantExitCode: 0,
			wantStdout: `
				osv-scanner version: dev
				commit: n/a
				built at: n/a
			`,
			wantStderr: "",
		},
		// one specific supported lockfile
		{
			name:         "",
			args:         []string{"", "./fixtures/locks-many/composer.lock"},
			wantExitCode: 0,
			wantStdout: `
				Scanning dir ./fixtures/locks-many/composer.lock
        Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
			`,
			wantStderr: "",
		},
		// one specific unsupported lockfile
		{
			name:         "",
			args:         []string{"", "./fixtures/locks-many/not-a-lockfile.toml"},
			wantExitCode: 128,
			wantStdout: `
				Scanning dir ./fixtures/locks-many/not-a-lockfile.toml
			`,
			wantStderr: `
				No package sources found, --help for usage information.
			`,
		},
		// all supported lockfiles in the directory should be checked
		{
			name:         "",
			args:         []string{"", "./fixtures/locks-many"},
			wantExitCode: 0,
			wantStdout: `
				Scanning dir ./fixtures/locks-many
				Scanned %%/fixtures/locks-many/Gemfile.lock file and found 1 packages
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
				Scanned %%/fixtures/locks-many/yarn.lock file and found 1 packages
			`,
			wantStderr: "",
		},
		// all supported lockfiles in the directory should be checked
		{
			name:         "",
			args:         []string{"", "./fixtures/locks-many-with-invalid"},
			wantExitCode: 127,
			wantStdout: `
				Scanning dir ./fixtures/locks-many-with-invalid
				Scanned %%/fixtures/locks-many-with-invalid/Gemfile.lock file and found 1 packages
				Scanned %%/fixtures/locks-many-with-invalid/yarn.lock file and found 1 packages
			`,
			wantStderr: `
				Attempted to scan file but failed: %%/fixtures/locks-many-with-invalid/composer.lock
			`,
		},
		// only the files in the given directories are checked by default (no recursion)
		{
			name:         "",
			args:         []string{"", "./fixtures/locks-one-with-nested"},
			wantExitCode: 0,
			wantStdout: `
				Scanning dir ./fixtures/locks-one-with-nested
				Scanned %%/fixtures/locks-one-with-nested/yarn.lock file and found 1 packages
			`,
			wantStderr: "",
		},
		// nested directories are checked when `--recursive` is passed
		{
			name:         "",
			args:         []string{"", "--recursive", "./fixtures/locks-one-with-nested"},
			wantExitCode: 0,
			wantStdout: `
				Scanning dir ./fixtures/locks-one-with-nested
				Scanned %%/fixtures/locks-one-with-nested/nested/composer.lock file and found 1 packages
				Scanned %%/fixtures/locks-one-with-nested/yarn.lock file and found 1 packages
			`,
			wantStderr: "",
		},
		// output with json
		{
			name:         "",
			args:         []string{"", "--json", "./fixtures/locks-many/composer.lock"},
			wantExitCode: 0,
			wantStdout: `
				{
					"results": []
				}
			`,
			wantStderr: `
				Scanning dir ./fixtures/locks-many/composer.lock
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
			`,
		},
		{
			name:         "",
			args:         []string{"", "--format", "json", "./fixtures/locks-many/composer.lock"},
			wantExitCode: 0,
			wantStdout: `
				{
					"results": []
				}
			`,
			wantStderr: `
				Scanning dir ./fixtures/locks-many/composer.lock
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
			`,
		},
		// output format: markdown table
		{
			name:         "",
			args:         []string{"", "--format", "markdown", "./fixtures/locks-many/composer.lock"},
			wantExitCode: 0,
			wantStdout: `
				Scanning dir ./fixtures/locks-many/composer.lock
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
			`,
			wantStderr: "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testCli(t, tt)
		})
	}
}

func TestRun_ParseAs(t *testing.T) {
	t.Parallel()

	tests := []cliTestCase{
		// invalid parse-as
		{
			name:         "",
			args:         []string{"", "--parse-as", "yarn.lock"},
			wantExitCode: 127,
			wantStdout:   "",
			wantStderr: `
				parse-as should be formatted as <parser>:<file> (got "yarn.lock")
			`,
		},
		// unsupported parse-as
		{
			name:         "",
			args:         []string{"", "--parse-as", "my-file:my-file"},
			wantExitCode: 127,
			wantStdout:   "",
			wantStderr: `
				Don't know how to parse files as "my-file" - supported values are:
					buildscript-gradle.lockfile
					Cargo.lock
					composer.lock
					conan.lock
					Gemfile.lock
					go.mod
					gradle.lockfile
					mix.lock
					package-lock.json
					packages.lock.json
					Pipfile.lock
					pnpm-lock.yaml
					poetry.lock
					pom.xml
					pubspec.lock
					requirements.txt
					yarn.lock

			`,
		},
		// technically valid, but currently meaningless
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"package-lock.json:",
				"-L",
				filepath.FromSlash("./fixtures/locks-insecure/my-package-lock.json"),
			},
			wantExitCode: 127,
			wantStdout:   "",
			wantStderr: `
				could not determine parser for %%/fixtures/locks-insecure/my-package-lock.json
			`,
		},
		{
			name:         "",
			args:         []string{"", "--parse-as", ":my-file"},
			wantExitCode: 127,
			wantStdout:   "",
			wantStderr: `
				Don't know how to parse files as "." - supported values are:
					buildscript-gradle.lockfile
					Cargo.lock
					composer.lock
					conan.lock
					Gemfile.lock
					go.mod
					gradle.lockfile
					mix.lock
					package-lock.json
					packages.lock.json
					Pipfile.lock
					pnpm-lock.yaml
					poetry.lock
					pom.xml
					pubspec.lock
					requirements.txt
					yarn.lock

			`,
		},
		// when a path to a file is given, parse-as is applied to that file
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"package-lock.json:./fixtures/locks-insecure/my-package-lock.json",
				filepath.FromSlash("./fixtures/locks-insecure/my-package-lock.json"),
			},
			wantExitCode: 1,
			wantStdout: `
				Scanning dir ./fixtures/locks-insecure/my-package-lock.json
				Scanned %%/fixtures/locks-insecure/my-package-lock.json file as a package-lock.json and found 1 packages
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| OSV URL (ID IN BOLD)                | ECOSYSTEM | PACKAGE   | VERSION | SOURCE                                       |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| https://osv.dev/GHSA-whgm-jr23-g3j9 | npm       | ansi-html | 0.0.1   | fixtures/locks-insecure/my-package-lock.json |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
			`,
			wantStderr: "",
		},
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"package-lock.json:./fixtures/locks-insecure/my-package-lock.json",
				"-L", filepath.FromSlash("./fixtures/locks-insecure/my-package-lock.json"),
			},
			wantExitCode: 1,
			wantStdout: `
				Scanned %%/fixtures/locks-insecure/my-package-lock.json file as a package-lock.json and found 1 packages
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| OSV URL (ID IN BOLD)                | ECOSYSTEM | PACKAGE   | VERSION | SOURCE                                       |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| https://osv.dev/GHSA-whgm-jr23-g3j9 | npm       | ansi-html | 0.0.1   | fixtures/locks-insecure/my-package-lock.json |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
			`,
			wantStderr: "",
		},
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"package-lock.json:./fixtures/locks-insecure/my-package-lock.json",
				filepath.FromSlash("./fixtures/locks-insecure"),
			},
			wantExitCode: 1,
			wantStdout: `
				Scanning dir ./fixtures/locks-insecure
				Scanned %%/fixtures/locks-insecure/composer.lock file and found 0 packages
				Scanned %%/fixtures/locks-insecure/my-package-lock.json file as a package-lock.json and found 1 packages
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| OSV URL (ID IN BOLD)                | ECOSYSTEM | PACKAGE   | VERSION | SOURCE                                       |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
				| https://osv.dev/GHSA-whgm-jr23-g3j9 | npm       | ansi-html | 0.0.1   | fixtures/locks-insecure/my-package-lock.json |
				+-------------------------------------+-----------+-----------+---------+----------------------------------------------+
			`,
			wantStderr: "",
		},
		// files that error on parsing don't stop parsable files from being checked
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"Cargo.lock:./fixtures/locks-insecure/my-package-lock.json",
				filepath.FromSlash("./fixtures/locks-insecure"),
				filepath.FromSlash("./fixtures/locks-many"),
			},
			wantExitCode: 127,
			wantStdout: `
				Scanning dir ./fixtures/locks-insecure
				Scanned %%/fixtures/locks-insecure/composer.lock file and found 0 packages
				Scanning dir ./fixtures/locks-many
				Scanned %%/fixtures/locks-many/Gemfile.lock file and found 1 packages
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
				Scanned %%/fixtures/locks-many/yarn.lock file and found 1 packages
			`,
			wantStderr: `
				Attempted to scan file as a Cargo.lock but failed: %%/fixtures/locks-insecure/my-package-lock.json
			`,
		},
		// parse-as takes priority, even if it's wrong
		{
			name: "",
			args: []string{
				"",
				"--parse-as",
				"package-lock.json:./fixtures/locks-many/yarn.lock",
				filepath.FromSlash("./fixtures/locks-many"),
			},
			wantExitCode: 127,
			wantStdout: `
				Scanning dir ./fixtures/locks-many
				Scanned %%/fixtures/locks-many/Gemfile.lock file and found 1 packages
				Scanned %%/fixtures/locks-many/composer.lock file and found 1 packages
			`,
			wantStderr: `
				Attempted to scan file as a package-lock.json but failed: %%/fixtures/locks-many/yarn.lock
			`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testCli(t, tt)
		})
	}
}
