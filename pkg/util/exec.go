package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunLocal executes a local command
func RunLocal(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// RunLocalShell executes a command via shell
func RunLocalShell(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("shell command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// CommandExists checks if a command is available
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
