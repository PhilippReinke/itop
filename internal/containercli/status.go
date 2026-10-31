package containercli

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// ServiceState describes whether the Apple container API server is available.
type ServiceState uint8

const (
	ServiceChecking ServiceState = iota
	ServiceRunning
	ServiceStopped
	ServiceUnknown
)

// CheckService handles both the current and 1.0 status JSON formats.
// The CLI may return valid status JSON with a nonzero exit when stopped.
func CheckService(runner Runner) (ServiceState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	output, runErr := runner.Run(ctx, "system", "status", "--format", "json")
	var payload struct {
		Status           string `json:"status"`
		APIServerVersion string `json:"apiServerVersion"`
		APIServerCommit  string `json:"apiServerCommit"`
		APIServerBuild   string `json:"apiServerBuild"`
		APIServerAppName string `json:"apiServerAppName"`
	}
	if json.NewDecoder(strings.NewReader(string(output))).Decode(&payload) == nil {
		switch strings.ToLower(payload.Status) {
		case "running":
			return ServiceRunning, nil
		case "not running", "notrunning", "stopped", "unregistered":
			return ServiceStopped, nil
		}
		if payload.APIServerVersion != "" || payload.APIServerCommit != "" || payload.APIServerBuild != "" || payload.APIServerAppName != "" {
			return ServiceRunning, nil
		}
		if strings.Contains(string(output), "apiServerVersion") {
			return ServiceStopped, nil
		}
	}

	message := strings.ToLower(strings.TrimSpace(string(output)))
	if runErr != nil {
		message += " " + strings.ToLower(runErr.Error())
	}
	for _, phrase := range []string{"not running", "notrunning", "stopped", "unregistered", "not registered with launchd"} {
		if strings.Contains(message, phrase) {
			return ServiceStopped, nil
		}
	}
	if runErr != nil {
		return ServiceUnknown, runErr
	}
	return ServiceRunning, nil
}
