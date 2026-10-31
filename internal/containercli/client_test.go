package containercli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeAppleResourceShapes(t *testing.T) {
	tests := []struct {
		kind   string
		id     string
		state  string
		detail string
	}{
		{"containers", "web", "running", "docker.io/library/nginx:latest"},
		{"images", "docker.io/library/alpine:latest", "", "sha256:abcd"},
		{"networks", "private", "", "192.168.100.0/24"},
		{"volumes", "db", "1048576", "local"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.kind+".json"))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := decodeResources(data, tt.kind)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].ID != tt.id || rows[0].State != tt.state || rows[0].Detail != tt.detail {
				t.Fatalf("rows = %#v", rows)
			}
		})
	}
}

func TestUnknownResourceShapeIsError(t *testing.T) {
	_, err := decodeResources([]byte(`[{"unexpected":"value"}]`), "containers")
	if err == nil {
		t.Fatal("unknown resource was silently dropped")
	}
}

func TestCommandArguments(t *testing.T) {
	if got := listArgs("containers"); !reflect.DeepEqual(got, []string{"list", "--all", "--format", "json"}) {
		t.Fatal(got)
	}
	if got := DeleteArgs("images", "alpine:latest"); !reflect.DeepEqual(got, []string{"image", "delete", "alpine:latest"}) {
		t.Fatal(got)
	}
}

type statusRunner struct {
	output []byte
	err    error
}

func (f statusRunner) Run(context.Context, ...string) ([]byte, error) {
	return f.output, f.err
}

func TestCheckService(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   ServiceState
	}{
		{"running", `{"status":"running"}`, nil, ServiceRunning},
		{"stopped 1.0", `{"apiServerVersion":"","status":"not running"}`, errors.New("exit status 1"), ServiceStopped},
		{"unregistered current", `{"status":"unregistered"}`, errors.New("exit status 1"), ServiceStopped},
		{"denied", "", errors.New("Operation not permitted"), ServiceUnknown},
		{"unknown response", `{}`, nil, ServiceUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := CheckService(statusRunner{[]byte(tt.output), tt.err})
			if got != tt.want {
				t.Fatalf("state = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCLIProvidesJSONOnFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "container")
	contents := "#!/bin/sh\nprintf '%s\\n' '{\"status\":\"not running\"}'\nexit 1\n"
	if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := (CLI{}).Run(context.Background(), "system", "status", "--format", "json")
	if err == nil || !strings.Contains(string(output), `"not running"`) {
		t.Fatalf("output = %q, error = %v", output, err)
	}
}
