# itop

A k9s-inspired TUI for
[Apple container](https://github.com/apple/container), built with Go and
Bubble Tea. It uses the installed `container` command and JSON list output.

## Install

Requires Apple container and Go 1.24 or newer:

```sh
go run ./cmd/itop
# or: go build -o itop ./cmd/itop && ./itop
```

Run `go run ./cmd/itop --help` for command line help. The root `main.go`
also supports `go run main.go`. Once this structure is published, install
with:

```sh
go install github.com/philippreinke/itop/cmd/itop@latest
```

The code uses `cmd/itop` for the entry point and `internal/app` for startup.
`internal/containercli` handles commands and JSON. `internal/ui` handles the
Bubble Tea model and rendering. See [TESTING.md](TESTING.md) for checks.

## Keys

| Key | Action |
| --- | --- |
| `j`/`k`, arrows | Move selection |
| `/` | Filter current view |
| `:` | Open `:containers`, `:images`, `:networks`, or `:volumes` |
| `Esc` | Back, or close the current panel or filter |
| `Enter` or `i` | Inspect selected resource |
| `l` / `v` | Show container logs / one-shot stats |
| `s` / `x` | Start / stop container |
| `e` | Open a shell in a container |
| `n` | Run image, pull image, or create network/volume |
| `d` | Delete selected resource (asks for confirmation) |
| `Enter` or `s` when stopped | Start container service |
| `S` | Start container service from any view |
| `r` | Refresh; `?` help; `q` quit |

The run prompt accepts an image reference and starts it detached. For more
flags, use the `container` CLI. `itop` refreshes the list every four seconds.
The logs panel refreshes on the same interval while open.

When the service is stopped, itop offers a start action. Starting it hands
the terminal to `container system start` for any setup prompts, then returns
to the TUI.
