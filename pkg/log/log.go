package log

import (
	"fmt"
	"os"
)

func Debug(a ...interface{}) error {
	logfile := "/var/log/console.log"
	f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	log := fmt.Sprintln(a...)
	if _, err = f.WriteString(log); err != nil {
		return err
	}

	return nil
}
