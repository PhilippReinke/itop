package containercli

import (
	"reflect"
	"testing"
)

func TestDecodeResources(t *testing.T) {
	tests := []struct{ kind, json, wantID, wantName string }{
		{"containers", `[{"id":"abc","name":"web","status":"running","image":"nginx"}]`, "abc", "web"},
		{"images", `[{"reference":"alpine:latest","size":1234}]`, "alpine:latest", "alpine:latest"},
		{"networks", `{"items":[{"name":"private","driver":"vmnet"}]}`, "private", "private"},
		{"volumes", `[{"Name":"db","Size":123}]`, "db", "db"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got, err := decodeResources([]byte(tt.json), tt.kind)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].ID != tt.wantID || got[0].Name != tt.wantName {
				t.Fatalf("got %#v", got)
			}
		})
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
