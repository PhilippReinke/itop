# itop

A k9s-inspired, keyboard driven TUI for [Apple container](https://github.com/apple/container), built with Go and Bubble Tea. It uses the installed `container` command and its JSON list output.

## Install

Requires Apple container installed and its system service running. Build with Go 1.24 or newer:

```sh
go run main.go
# or: go build -o itop . && ./itop
```

Run `go run main.go --help` for command line help. You can also install the published module with `go install github.com/philippreinke/itop@latest`.

## Keys

| Key | Action |
| --- | --- |
| `j`/`k`, arrows | Move selection |
| `/` | Filter current view |
| `:` | Open a resource by name or alias (`:containers`, `:images`, `:networks`, `:volumes`) |
| `Esc` | Return to the previous resource, or close the current panel/filter |
| `Enter` or `i` | Inspect selected resource |
| `l` / `v` | Show container logs / one-shot stats |
| `s` / `x` | Start / stop container |
| `e` | Open a shell in a container |
| `n` | Run an image, pull an image, or create a network/volume |
| `d` | Delete selected resource (asks for confirmation) |
| `Enter` or `s` when stopped | Start the Apple container service interactively |
| `S` | Start the Apple container service from any view |
| `r` | Refresh; `?` help; `q` quit |

The run prompt accepts an image reference and starts it detached. For advanced flags, use the `container` CLI directly. `itop` refreshes the current list every four seconds.

When the Apple container service is stopped, itop shows a start action in the resource view. Starting the service temporarily hands the terminal to `container system start` so you can answer any setup prompts, then returns to the TUI.
