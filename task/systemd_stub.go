//go:build !linux

package task

import "fmt"

func InstallSystemdTimer(t Task) error {
	return fmt.Errorf("systemd timers are only supported on Linux")
}

func RemoveSystemdTimer(t Task) error {
	return fmt.Errorf("systemd timers are only supported on Linux")
}
