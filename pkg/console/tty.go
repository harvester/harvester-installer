package console

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

func getFirstConsoleTTY() string {
	b, err := ioutil.ReadFile("/sys/class/tty/console/active")
	if err != nil {
		logrus.Error(err)
		return ""
	}

	ttys := strings.Split(strings.TrimRight(string(b), "\n"), " ")
	if len(ttys) > 0 {
		return ttys[0]
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
