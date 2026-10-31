# Testing itop

Run the automated checks after changes:

```sh
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l .)"
```

The tests use captured shapes of Apple container JSON and a fake command runner. They cover service status (including a nonzero exit when stopped), list parsing, command arguments, confirmation, filter cancellation, stale view responses, and rendering inside small terminal sizes. Add fixtures when a supported `container` release changes JSON; avoid testing against guessed field names. An unknown shape should produce an error instead of an empty list.

For a fuller regression gate, create a fake `container` executable in a temporary `PATH` and run the built binary in a pseudo terminal. Check the actual key sequences and screen output, especially switching views while refresh is pending and returning from interactive commands. Keep those tests limited to flows that unit tests cannot cover.

Before a release, smoke test against a real Apple container installation:

| Setup | Check |
| --- | --- |
| Service stopped | Start offer appears; Enter runs `container system start`; list refreshes afterward |
| Service running, no resources | Empty state and footer remain visible |
| Running and stopped containers | Navigation, inspect, logs, stats, start, stop, shell |
| Images, networks, volumes | Lists, inspect, create or pull, delete confirmation |
| Narrow and resized terminal | Prompt, status, and footer stay on screen in each mode |
| CLI error or unexpected JSON | Error is visible and refresh can recover |

`go run ./cmd/itop --help` and `go run main.go --help` should both work. The real service and interactive shell need a macOS host with Apple container installed; the automated suite does not require them.
