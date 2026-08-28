package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Run(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func EnsureRepo(repo string) error {
	_, err := Run(repo, "rev-parse", "--is-inside-work-tree")
	return err
}
