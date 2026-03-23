package daemon

import (
	"fmt"
	"os/exec"
	"runtime"
)

// SendNotification sends a native OS notification.
func SendNotification(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		sendMacNotification(title, message)
	case "linux":
		sendLinuxNotification(title, message)
	}
}

func sendMacNotification(title, message string) {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	exec.Command("osascript", "-e", script).Run()
}

func sendLinuxNotification(title, message string) {
	exec.Command("notify-send", title, message).Run()
}
