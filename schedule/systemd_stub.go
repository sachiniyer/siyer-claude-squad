//go:build !linux

package schedule

import "fmt"

func InstallSystemdTimer(s Schedule) error {
	return fmt.Errorf("systemd timers are only supported on Linux")
}

func RemoveSystemdTimer(s Schedule) error {
	return fmt.Errorf("systemd timers are only supported on Linux")
}
