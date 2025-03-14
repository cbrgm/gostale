# 🧼 gostale

**`gostale` tracks and enforces `TODOs` and `FIXMEs` with expiration dates in Go code.**

[![GitHub release](https://img.shields.io/github/release/cbrgm/gostale.svg)](https://github.com/cbrgm/gostale)
[![Go Report Card](https://goreportcard.com/badge/github.com/cbrgm/gostale)](https://goreportcard.com/report/github.com/cbrgm/gostale)
[![go-lint-test](https://github.com/cbrgm/gostale/actions/workflows/go-lint-test.yml/badge.svg)](https://github.com/cbrgm/gostale/actions/workflows/go-lint-test.yml)
[![go-binaries](https://github.com/cbrgm/gostale/actions/workflows/go-binaries.yml/badge.svg)](https://github.com/cbrgm/gostale/actions/workflows/go-binaries.yml)
[![container](https://github.com/cbrgm/gostale/actions/workflows/container.yml/badge.svg)](https://github.com/cbrgm/gostale/actions/workflows/container.yml)

- [🧼 gostale](#---gostale)
  * [What is it?](#what-is-it-)
  * [Why use it?](#why-use-it-)
  * [How to use it](#how-to-use-it)
      - [Run using `go tool`](#run-using--go-tool-)
      - [Download the Binary](#download-the-binary)
      - [Install From Source](#install-from-source)
      - [Container Usage](#container-usage)
      - [GitHub Actions](#github-actions)
  * [Examples](#examples)
  * [Contributing & License](#contributing---license)

## What is it?

**gostale** helps you manage technical debt by scanning Go files for `todo` and `fixme` comments that include a `stale:` date (and optionally an `expires:` date and message). When a date is reached or passed, it can warn or fail — making TODOs actionable.

Example:
```go
// todo(cbrgm): stale:01-03-2025 expires:01-04-2025 implement a new AuthHandler
func oldAuth() {}
```

## Why use it?

- ✅ **Make TODOs count** — Add real deadlines to your `TODO` and `FIXME` comments.
- 🛑 **Prevent forgotten work** — Expired comments can fail your CI (if you want 😄).
- 🧹 **Enforce cleanup culture** — Track stale code before it turns to rot.
- 📌 **Zero friction** — Just plain Go comments. No custom tags or weird syntax.
- ⚙️ **Flexible** — Supports custom date formats, default expiry windows, log levels, and directory filters.

## How to use it

The `gostale` binary supports the following flags:

- `--today`: Optional - Override today’s date in `DD-MM-YYYY` format. Can also be set via the `GOSTALE_DATE` environment variable.
- `--exclude`: Optional - Comma-separated list of directories to skip (e.g., `vendor,third_party`).
- `--fail-on-expired`: Optional - Exit with code `1` if any expired annotations are found.
- `--log-level`: Optional - Set log verbosity. Options: `debug`, `info`, `warn`, `error`. Default is `info`.
- `--default-expiry-days`: Optional - Default number of days after the `stale` date to consider code expired. Default is `90`.
- `--date-format`: Optional - Custom date format for parsing `stale` and `expires` fields. Default is `02-01-2006` (DD-MM-YYYY).

**Positional arguments:**
- `PATH`: Path or Go package pattern (e.g., `./...`) to scan. Defaults to current directory.

#### Run using `go tool`

You can use **gostale** via the `go tool` directive introduced in Go 1.24:

```bash
go tool github.com/cbrgm/gostale@latest
go tool run gostale .
```

#### Download the Binary

1) Download from [Releases](https://github.com/cbrgm/gostale/releases).
2) Pick the right binary for your OS.
3) Place it in your `$PATH`.

#### Install From Source

You can build **gostale** from sourc using the following commands:

```bash
git clone https://github.com/cbrgm/gostale.git && cd gostale
make build && ./bin/gostale -h
```

or via `go install`.

#### Container Usage

**gostale** can be executed independently from workflows within a container. To do so, use the following command:

```bash
podman run --rm -v $(pwd):/code ghcr.io/cbrgm/gostale:v1 /code
```

#### GitHub Actions

**Inputs**

- `path`: Optional - Path or package pattern to analyze (e.g., `./...`). Defaults to `.`.
- `today`: Optional - Override today’s date (format: `DD-MM-YYYY`). Useful for deterministic testing.
- `exclude`: Optional - Comma-separated list of directories to exclude from scanning.
- `fail-on-expired`: Optional - Exit with code 1 if expired annotations are found. Accepts `true` or `false`. Defaults to `false`.
- `log-level`: Optional - Specifies the logging level (`debug`, `info`, `warn`, `error`). Defaults to `info`.

**Example Workflow**:
```yaml
name: Check for stale annotations

on:
  pull_request:
    paths:
      - '**.go'

jobs:
  gostale:
    name: Run gostale
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run gostale
        uses: cbrgm/gostale@v1
        with:
          path: ./...
          fail-on-expired: true
          log-level: warn

```

## Examples

**Long**

```go
// todo(cbrgm): stale:01-03-2025 expires:01-04-2025 implement a new AuthHandler
func oldAuth() {}
```

**Short**
```go
// todo(cbrgm): stale:01-03-2025 implement a new AuthHandler
func oldAuth() {}
```

**Alternatives**
```go
// all of these are supported:
//
// todo: stale:01-01-2024
// fixme: stale:01-02-2024 expires:01-03-2024 Use new auth handler
// TODO(cbrgm): stale:10-03-2025 expires:09-06-2025 this is a test
// fixme(cbrgm): stale:01-01-2024 expires:01-04-2024 see https://example.com/refactor
// todo: stale:01-01-2024 improve this before release
func oldAuth() {}
```

**Output**
```bash
time=2025-03-10T23:32:12.805+01:00 level=ERROR msg="EXPIRED: /gostale/cmd/gostale/main.go:58 [main]" stale_date=01-01-2024 expires=31-03-2024 todo=""
time=2025-03-10T23:32:12.805+01:00 level=ERROR msg="EXPIRED: /gostale/cmd/gostale/main.go:59 [main]" stale_date=01-02-2024 expires=01-03-2024 todo="Use new auth handler"
time=2025-03-10T23:32:12.805+01:00 level=WARN msg="STALE: /gostale/cmd/gostale/main.go:60 [main]" stale_date=10-03-2025 expires=09-06-2025 todo="this is a test"
time=2025-03-10T23:32:12.805+01:00 level=ERROR msg="EXPIRED: /gostale/cmd/gostale/main.go:61 [main]" stale_date=01-01-2024 expires=01-04-2024 todo="see https://example.com/refactor"
time=2025-03-10T23:32:12.805+01:00 level=ERROR msg="EXPIRED: /gostale/cmd/gostale/main.go:62 [main]" stale_date=01-01-2024 expires=31-03-2024 todo="improve this before release"
```

## Contributing & License

* **Contributions Welcome!**: Interested in improving or adding features? Check our [Contributing Guide](https://github.com/cbrgm/gostale/blob/main/CONTRIBUTING.md) for instructions on submitting changes and setting up development environment.
* **Open-Source & Free**: Developed in my spare time, available for free under [Apache 2.0 License](https://github.com/cbrgm/gostale/blob/main/LICENSE). License details your rights and obligations.
* **Your Involvement Matters**: Code contributions, suggestions, feedback crucial for improvement and success. Let's maintain it as a useful resource for all 🌍.
