//go:build windows

package remote

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

func restrictPrivateKey(path string) error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	account := strings.TrimSpace(current.Username)
	if account == "" {
		return fmt.Errorf("无法确定当前 Windows 用户")
	}
	output, err := exec.Command("icacls", path, "/inheritance:r", "/grant:r", account+":(R)").CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
