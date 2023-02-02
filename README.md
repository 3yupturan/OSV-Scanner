# OSV-Scanner

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/google/osv-scanner/badge)](https://api.securityscorecards.dev/projects/github.com/google/osv-scanner)

Use OSV-Scanner to find existing vulnerabilities affecting your project's dependencies.

OSV-Scanner provides an officially supported frontend to the [OSV database](https://osv.dev/) that connects a project’s list of dependencies with the vulnerabilities that affect them. Since the OSV.dev database is open source and distributed, it has several benefits in comparison with closed source advisory databases and scanners:

- Each advisory comes from an open and authoritative source (e.g. the [RustSec Advisory Database](https://github.com/rustsec/advisory-db))
- Anyone can suggest improvements to advisories, resulting in a very high quality database
- The OSV format unambiguously stores information about affected versions in a machine-readable format that precisely maps onto a developer’s list of packages

The above all results in fewer, more actionable vulnerability notifications, which reduces the time needed to resolve them. Check out our [announcement blog post] for more details!

[announcement blog post]: https://security.googleblog.com/2022/12/announcing-osv-scanner-vulnerability.html

## Table of Contents

- [OSV-Scanner](#osv-scanner)
  - [Table of Contents](#table-of-contents)
  - [Installing](#installing)
    - [Package Managers](#package-managers)
    - [Install from source](#install-from-source)
    - [Build from source](#build-from-source)
    - [SemVer Adherence](#semver-adherence)
  - [Usage](#usage)
    - [Scan a directory](#scan-a-directory)
    - [Input an SBOM](#input-an-sbom)
    - [Input a lockfile](#input-a-lockfile)
    - [Scanning a Debian based docker image packages (preview)](#scanning-a-debian-based-docker-image-packages-preview)
    - [Running in a Docker Container](#running-in-a-docker-container)
  - [Configure OSV-Scanner](#configure-osv-scanner)
    - [Ignore vulnerabilities by ID](#ignore-vulnerabilities-by-id)
  - [JSON output](#json-output)
    - [Output Format](#output-format)
  - [Contribute](#contribute)
    - [Report Problems](#report-problems)
    - [Contributing code to `osv-scanner`](#contributing-code-to-osv-scanner)
  - [Stargazers over time](#stargazers-over-time)

## Installing

You may download the [SLSA3](https://slsa.dev) compliant binaries for Linux, macOS, and Windows from our [releases page](https://github.com/google/osv-scanner/releases).

### Package Managers

[![Packaging status](https://repology.org/badge/vertical-allrepos/osv-scanner.svg)](https://repology.org/project/osv-scanner/versions)

If you're a [**Windows Scoop**](https://scoop.sh) user, then you can install osv-scanner from the [official bucket](https://github.com/ScoopInstaller/Main/blob/master/bucket/osv-scanner.json):

```console
scoop install osv-scanner
```

If you're a [Homebrew](https://brew.sh/) user, you can install [osv-scanner](https://formulae.brew.sh/formula/osv-scanner) via:

```console
brew install osv-scanner
```

If you're a Arch Linux User, you can install osv-scanner from the official repo:
```
pacman -S osv-scanner
```

### Install from source

Alternatively, you can install this from source by running:

```console
go install github.com/google/osv-scanner/cmd/osv-scanner@v1
```

This requires Go 1.18+ to be installed.

### Build from source

See [CONTRIBUTING.md](CONTRIBUTING.md) file.

### SemVer Adherence

All releases on the same Major version will be guaranteed to have backward compatible JSON output and CLI arguments.

## Usage

OSV-scanner parses lockfiles, SBOMs, and git directories to determine your project's open source dependencies. These dependencies are matched against the OSV database via the [OSV.dev API](https://osv.dev#use-the-api) and known vulnerabilities are returned to you in the output. 

### General Use Case: Scanning a Directory

```console
osv-scanner -r /path/to/your/dir
```

The above command will find lockfiles, SBOMs, and git directories in your target directory and use them to determine the dependencies to check against the OSV database for any known vulnerabilities.

The recursive flag `-r` or `--recursive` will tell the scanner to search all subdirectories in addition to the specified directory. It can find additional lockfiles, dependencies, and vulnerabilities. If your project has deeply nested subdirectories, a recursive search may take a long time. 

Git directories are searched for the latest commit hash. Searching for git commit hash is intended to work with projects that use git submodules or a similar mechanism where dependencies are checked out as real git repositories. 

### Specify SBOM

If you want to check for known vulnerabilities only in dependencies in your SBOM, you can use the following command:

```console
osv-scanner --sbom=/path/to/your/sbom.json
```

[SPDX] and [CycloneDX] SBOMs using [Package URLs] are supported. The format is
auto-detected based on the input file contents.

[SPDX]: https://spdx.dev/
[CycloneDX]: https://cyclonedx.org/
[Package URLs]: https://github.com/package-url/purl-spec

### Specify Lockfile(s)
If you want to check for known vulnerabilities in specific lockfiles, you can use the following command:

```console
osv-scanner --lockfile=/path/to/your/package-lock.json --lockfile=/path/to/another/Cargo.lock
```

It is possible to specify more than one lockfile at a time. 

A wide range of lockfiles are supported by utilizing this [lockfile package](https://github.com/google/osv-scanner/tree/main/pkg/lockfile). This is the current list of supported lockfiles:

- `buildscript-gradle.lockfile`
- `Cargo.lock`
- `composer.lock`
- `conan.lock`
- `Gemfile.lock`
- `go.mod`
- `gradle.lockfile`
- `mix.lock`
- `package-lock.json`
- `packages.lock.json`
- `Pipfile.lock`
- `pnpm-lock.yaml`
- `poetry.lock`
- `pom.xml`[\*](https://github.com/google/osv-scanner/issues/35)
- `pubspec.lock`
- `requirements.txt`[\*](https://github.com/google/osv-scanner/issues/34)
- `yarn.lock`
- `/lib/apk/db/installed` (Alpine)

### Scanning a Debian based docker image packages (preview)

This tool will scrape the list of installed packages in a Debian image and query for vulnerabilities on them.

Currently only Debian based docker image scanning is supported.

Requires `docker` to be installed and the tool to have permission calling it.

This currently does not scan the filesystem of the Docker container, and has various other limitations. Follow [this issue](https://github.com/google/osv-scanner/issues/64) for updates on container scanning!

#### Example

```console
osv-scanner --docker image_name:latest
```

### Running in a Docker Container

The simplest way to get the osv-scanner docker image is to pull from GitHub Container Registry:

```bash
docker pull ghcr.io/google/osv-scanner:latest
```

Once you have the image, you can test that it works by running:

```bash
docker run -it ghcr.io/google/osv-scanner -h
```

Finally, to run it, mount the directory you want to scan to `/src` and pass the
appropriate osv-scanner flags:

```bash
docker run -it -v ${PWD}:/src ghcr.io/google/osv-scanner -L /src/go.mod
```

## Configure OSV-Scanner

To configure scanning, place an osv-scanner.toml file in the scanned file's directory. To override this osv-scanner.toml file, pass the `--config=/path/to/config.toml` flag with the path to the configuration you want to apply instead.

Currently, there is only 1 option to configure:

### Ignore vulnerabilities by ID

To ignore a vulnerability, enter the ID under the `IgnoreVulns` key. Optionally, add an expiry date or reason.

#### Example

```toml
[[IgnoredVulns]]
id = "GO-2022-0968"
# ignoreUntil = 2022-11-09 # Optional exception expiry date
reason = "No ssh servers are connected to or hosted in Go lang"

[[IgnoredVulns]]
id = "GO-2022-1059"
# ignoreUntil = 2022-11-09 # Optional exception expiry date
reason = "No external http servers are written in Go lang."
```

## Output formats

You can control the format used by the scanner to output results with the `--format` flag. The different formats supported by the scanner are:

### `table` format

The default format, which outputs the results as a human-readable table.

Sample output:

```
╭─────────────────────────────────────┬───────────┬──────────────────────────┬─────────┬────────────────────╮
│ OSV URL (ID IN BOLD)                │ ECOSYSTEM │ PACKAGE                  │ VERSION │ SOURCE             │
├─────────────────────────────────────┼───────────┼──────────────────────────┼─────────┼────────────────────┤
│ https://osv.dev/GHSA-c3h9-896r-86jm │ Go        │ github.com/gogo/protobuf │ 1.3.1   │ path/to/go.mod     │
│ https://osv.dev/GHSA-m5pq-gvj9-9vr8 │ crates.io │ regex                    │ 1.3.1   │ path/to/Cargo.lock │
╰─────────────────────────────────────┴───────────┴──────────────────────────┴─────────┴────────────────────╯
```

### `json` format

Outputs the results as a JSON object to stdout, with all other output being directed to stderr - this makes it safe to redirect the output to a file with `osv-scanner --format json ... > /path/to/file.json`.

Sample output:

```json5
{
  "results": [
    {
      "packageSource": {
        "path": "/absolute/path/to/go.mod",
        // One of: lockfile, sbom, git, docker
        "type": "lockfile"
      },
      "packages": [
        {
          "package": {
            "name": "github.com/gogo/protobuf",
            "version": "1.3.1",
            "ecosystem": "Go"
          },
          "vulnerabilities": [
            {
              "id": "GHSA-c3h9-896r-86jm",
              "aliases": [
                "CVE-2021-3121"
              ],
              // ... Full OSV
            },
            {
              "id": "GO-2021-0053",
              "aliases": [
                "CVE-2021-3121",
                "GHSA-c3h9-896r-86jm"
              ],
              // ... Full OSV
            }
          ],
          // Grouping based on aliases, if two vulnerability share the same alias, or alias each other,
          // they are considered the same vulnerability, and is grouped here under the id field.
          "groups": [
            {
              "ids": [
                "GHSA-c3h9-896r-86jm",
                "GO-2021-0053"
              ]
            }
          ]
        }
      ]
    },
    {
      "packageSource": {
        "path": "/absolute/path/to/Cargo.lock",
        "type": "lockfile"
      },
      "packages": [
        {
          "package": {
            "name": "regex",
            "version": "1.5.1",
            "ecosystem": "crates.io"
          },
          "vulnerabilities": [
            {
              "id": "GHSA-m5pq-gvj9-9vr8",
              "aliases": [
                "CVE-2022-24713"
              ],
              // ... Full OSV
            },
            {
              "id": "RUSTSEC-2022-0013",
              "aliases": [
                "CVE-2022-24713"
              ],
              // ... Full OSV
            }
          ],
          "groups": [
            {
              "ids": [
                "GHSA-m5pq-gvj9-9vr8",
                "RUSTSEC-2022-0013"
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

## Contribute

### Report Problems
If you have what looks like a bug, please use the [Github issue tracking system](https://github.com/google/osv-scanner/issues). Before you file an issue, please search existing issues to see if your issue is already covered.

### Contributing code to `osv-scanner`

See [CONTRIBUTING.md](CONTRIBUTING.md) for documentation on how to contribute code.


## Stargazers over time

[![Stargazers over time](https://starchart.cc/google/osv-scanner.svg)](https://starchart.cc/google/osv-scanner)
