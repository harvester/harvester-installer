package widgets

import (
	"github.com/jroimartin/gocui"
)

func ArrowUp(_ *gocui.Gui, v *gocui.View) error {
	if v == nil || isAtTop(v) {
		return nil
	}

	cx, cy := v.Cursor()
	if err := v.SetCursor(cx, cy-1); err != nil {
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy-1); err != nil {
			return err
		}
	}
	return nil
}

func ArrowDown(_ *gocui.Gui, v *gocui.View) error {
	if v == nil || isAtEnd(v) {
		return nil
	}
	cx, cy := v.Cursor()
	if err := v.SetCursor(cx, cy+1); err != nil {
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy+1); err != nil {
			return err
		}
	}

	return nil
}

func isAtTop(v *gocui.View) bool {
	_, cy := v.Cursor()
	if cy == 0 {
		return true
	}
	return false
}

func isAtEnd(v *gocui.View) bool {
	_, cy := v.Cursor()
	lines := len(v.BufferLines())
	if lines < 2 || cy == lines-2 {
		return true
	}
	return false
}

// This will insert linebreaks in a string so that it doesn't exceed
// the specified number of columns.
func wrapText(text string, columns int) string {
	if len(text) < columns {
		return text
	}
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == ' ' && i < columns {
			return text[:i] + "\n" + wrapText(text[i+1:], columns)
		}
	}
	return text
}
