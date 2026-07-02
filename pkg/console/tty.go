package console

import (
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func isVirtualTTY(tty string) bool {
	if !strings.HasPrefix(tty, "tty") {
		return false
	}

	ttyNum, err := strconv.Atoi(strings.TrimPrefix(tty, "tty"))
	return err == nil && ttyNum > 0
}

func getPreferredConsoleTTY(ttys []string) string {
	preferredVirtualTTY := ""

	for _, tty := range ttys {
		if tty == "tty1" {
			return tty
		}
		if preferredVirtualTTY == "" && isVirtualTTY(tty) {
			preferredVirtualTTY = tty
		}
	}

	if preferredVirtualTTY != "" {
		return preferredVirtualTTY
	}

	for _, tty := range ttys {
		if tty != "tty0" {
			return tty
		}
	}
	return ttys[0]
}

func getFirstConsoleTTY() string {
	b, err := os.ReadFile("/sys/class/tty/console/active")
	if err != nil {
		logrus.Error(err)
		return ""
	}

	ttys := strings.Split(strings.TrimRight(string(b), "\n"), " ")
	if len(ttys) > 0 {
		return getPreferredConsoleTTY(ttys)
	}
	return ""
}

func isFirstConsoleTTY() bool {
	tty := os.Getenv("TTY")
	logrus.Infof("my tty is %s", tty)
	if tty == "" {
		return false
	}
	return strings.TrimPrefix(tty, "/dev/") == getFirstConsoleTTY()
}
