# Testing itop

Run these checks after changes:

```sh
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Tests use Apple container JSON fixtures and a fake command runner. They cover
service status, list parsing, command arguments, confirmation, filter
cancellation, stale responses, and rendering at small terminal sizes. Add
fixtures when a supported `container` release changes JSON. Unknown shapes
should produce an error instead of an empty list.

For a fuller regression gate, put a fake `container` executable in a
temporary `PATH` and run the built binary in a pseudo terminal. Check real
key sequences and screen output, especially view switches during refresh and
returning from interactive commands.

Before a release, smoke test on a real Apple container installation:

| Setup | Check |
| --- | --- |
| Service stopped | Start offer, Enter starts service, list refreshes |
| Service running, empty list | Empty state and footer stay visible |
| Running and stopped containers | Navigate, inspect, logs, stats, shell |
| Shell exits | Frame, border, and footer redraw after terminal handoff |
| Logs stay open | New lines appear; scrolling up keeps the current position |
| Images, networks, volumes | List, inspect, create/pull, delete |
| Narrow or resized terminal | Prompt, status, footer, borders stay visible |
| CLI error or unexpected JSON | Error is visible; refresh can recover |

Both `go run ./cmd/itop --help` and `go run main.go --help` should work.
The real service and interactive shell need a macOS host with Apple container
installed. The automated suite does not require them.
