package containercli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Resource is the small set of fields needed by the list view.
type Resource struct {
	ID     string
	Name   string
	State  string
	Detail string
}

func List(runner Runner, kind string) ([]Resource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	output, err := runner.Run(ctx, listArgs(kind)...)
	if err != nil {
		return nil, err
	}
	return decodeResources(output, kind)
}

func decodeResources(data []byte, kind string) ([]Resource, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode %s list: %w", kind, err)
	}

	items, ok := value.([]any)
	if !ok {
		if wrapper, isObject := value.(map[string]any); isObject {
			for _, key := range []string{"items", kind} {
				if candidate, isArray := lookup(wrapper, key).([]any); isArray {
					items, ok = candidate, true
					break
				}
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("unexpected %s list JSON: expected an array", kind)
	}

	resources := make([]Resource, 0, len(items))
	for index, item := range items {
		object, isObject := item.(map[string]any)
		if !isObject {
			return nil, fmt.Errorf(
				"unexpected %s item %d: expected an object", kind, index,
			)
		}
		resource := parseResource(object, kind)
		if resource.ID == "" {
			return nil, fmt.Errorf("unexpected %s item %d: no resource ID", kind, index)
		}
		if resource.Name == "" {
			resource.Name = resource.ID
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func parseResource(object map[string]any, kind string) Resource {
	var resource Resource
	switch kind {
	case "containers":
		resource.ID = first(
			field(object, "id", "containerID", "identifier"),
			path(object, "configuration", "id"),
		)
		resource.Name = first(field(object, "name", "names"), resource.ID)
		resource.State = first(
			path(object, "status", "state"),
			field(object, "status", "state"),
		)
		resource.Detail = first(
			path(object, "configuration", "image", "reference"),
			field(object, "image", "imageName"),
		)
	case "images":
		resource.ID = first(
			path(object, "configuration", "name"),
			field(
				object, "reference", "displayReference", "name", "id",
			),
		)
		resource.Name, resource.State = imageNameAndTag(resource.ID)
		resource.Detail = first(
			path(object, "configuration", "descriptor", "digest"),
			path(object, "descriptor", "digest"),
			field(object, "digest", "id"),
		)
	case "networks":
		resource.ID = field(object, "id", "name")
		resource.Name = resource.ID
		resource.State = field(object, "driver", "plugin")
		resource.Detail = first(
			path(object, "status", "ipv4Subnet"),
			field(object, "subnet", "ipv4Subnet"),
		)
	case "volumes":
		resource.ID = field(object, "name", "id")
		resource.Name = resource.ID
		resource.State = field(object, "sizeInBytes", "size", "sizeBytes")
		resource.Detail = field(object, "driver", "mountpoint")
	}
	return resource
}

func imageNameAndTag(reference string) (string, string) {
	name := reference
	tag := "<none>"
	if at := strings.LastIndex(name, "@"); at >= 0 {
		name = name[:at]
	} else {
		colon := strings.LastIndex(name, ":")
		if colon > strings.LastIndex(name, "/") {
			name, tag = name[:colon], name[colon+1:]
		}
	}
	name = strings.TrimPrefix(name, "docker.io/library/")
	name = strings.TrimPrefix(name, "docker.io/")
	return name, tag
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func path(object map[string]any, keys ...string) string {
	for _, key := range keys[:len(keys)-1] {
		child, ok := lookup(object, key).(map[string]any)
		if !ok {
			return ""
		}
		object = child
	}
	return field(object, keys[len(keys)-1])
}

func lookup(object map[string]any, key string) any {
	for name, value := range object {
		if strings.EqualFold(name, key) {
			return value
		}
	}
	return nil
}

func field(object map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := lookup(object, key).(type) {
		case string:
			if value != "" {
				return value
			}
		case json.Number:
			return value.String()
		case []any:
			if len(value) > 0 {
				return fmt.Sprint(value[0])
			}
		}
	}
	return ""
}

func listArgs(kind string) []string {
	if kind == "containers" {
		return []string{"list", "--all", "--format", "json"}
	}
	return []string{strings.TrimSuffix(kind, "s"), "list", "--format", "json"}
}

func InspectArgs(kind, id string) []string {
	if kind == "containers" {
		return []string{"inspect", id}
	}
	return []string{strings.TrimSuffix(kind, "s"), "inspect", id}
}

func DeleteArgs(kind, id string) []string {
	if kind == "containers" {
		return []string{"delete", id}
	}
	return []string{strings.TrimSuffix(kind, "s"), "delete", id}
}

func PrettyJSON(data string) string {
	var value any
	if json.Unmarshal([]byte(data), &value) != nil {
		return data
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return data
	}
	return string(pretty)
}
