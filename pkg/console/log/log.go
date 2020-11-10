package log

import (
	"fmt"
	"os"

	"github.com/jroimartin/gocui"
)

func Debug(g *gocui.Gui, a ...interface{}) error {
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

	// v, err := g.SetView("debug", 0, 0, 40, 40)
	// v.Wrap = true
	// if err != nil && err != gocui.ErrUnknownView {
	// 	return err
	// }
	// _, err = fmt.Fprintln(v, a...)
	return nil
}
