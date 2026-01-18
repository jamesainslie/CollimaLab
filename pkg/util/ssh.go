package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// SSHConfig holds SSH connection details
type SSHConfig struct {
	Host string
	User string
}

// RunSSH executes a command on remote host via SSH
func RunSSH(cfg SSHConfig, command string) (string, error) {
	target := fmt.Sprintf("%s@%s", cfg.User, cfg.Host)
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", target, command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("ssh command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// TestSSH verifies SSH connectivity
func TestSSH(cfg SSHConfig) error {
	_, err := RunSSH(cfg, "echo ok")
	return err
}
