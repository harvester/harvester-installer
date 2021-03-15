package console

import (
	"os"

	"github.com/rancher/harvester-installer/pkg/util"
)

func getLastTTY() string {
	tty := os.Getenv("TTY")
	if tty == "" || tty == "/dev/tty1" {
		return ""
	}
	kernelParams, err := util.ReadCmdline("")
	if err != nil {
		return ""
	}
	if value, ok := kernelParams["console"]; ok {
		switch value.(type) {
		case []string:
			consoles := value.([]string)
			return consoles[len(consoles)-1]
		case string:
			return value.(string)
		}
	}
	return ""
}
