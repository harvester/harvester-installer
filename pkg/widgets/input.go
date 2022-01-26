package widgets

import (
	"github.com/jroimartin/gocui"
)

// Create an Editor wrapped the default Editor for updating the Value field of Input struct
// whenever the buffer is updated.
func createValueUpdateEditor(i *Input) gocui.EditorFunc {
	return func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
		gocui.DefaultEditor.Edit(v, key, ch, mod)

		if len(v.BufferLines()) == 0 {
			i.Value = ""
		}
		i.Value, _ = v.Line(0)
	}
}

type Input struct {
	*Panel
	Value string
	Mask  bool

	onConfirm EventCallback
	onLeave   EventCallback
}

func NewInput(g *gocui.Gui, name string, label string, mask bool) (*Input, error) {
	maxX, maxY := g.Size()
	return &Input{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: label,
			X0:      maxX / 8,
			Y0:      maxY / 8,
			X1:      maxX / 8 * 7,
			Y1:      maxY/8 + 3,
		},
		Mask: mask,
	}, nil
}

func (i *Input) Show() error {
	if err := i.Panel.Show(); err != nil {
		return err
	}
	inputViewName := i.Name + "-input"
	offset := 20
	if len(i.Content) > offset {
		offset = len(i.Content) + 1
	}
	x0 := i.X0 + offset
	x1 := i.X1 - 1
	y0 := i.Y0
	y1 := i.Y0 + 2
	v, err := i.g.SetView(inputViewName, x0, y0, x1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		v.Editor = gocui.EditorFunc(createValueUpdateEditor(i))
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		if i.Mask {
			v.Mask ^= '*'
		}

		i.setDefaultKeybindings()

		if i.KeyBindings != nil {
			for key, f := range i.KeyBindings {
				if err := i.g.SetKeybinding(inputViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
	}
	if _, err = i.g.SetCurrentView(inputViewName); err != nil {
		return err
	}

	return i.SetData(i.Value)
}

func (i *Input) Close() error {
	inputViewName := i.Name + "-input"
	i.g.DeleteKeybindings(inputViewName)
	if err := i.g.DeleteView(inputViewName); err != nil {
		return err
	}
	return i.Panel.Close()
}

func (i *Input) GetData() (string, error) {
	return i.Value, nil
}

func (i *Input) SetData(data string) error {
	inputViewName := i.Name + "-input"
	ov, err := i.g.View(inputViewName)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// View is not created yet, only update Value, don't edit the buffer
		i.Value = data
		return nil
	}

	i.Value = data
	ov.Clear()
	_, err = ov.Write([]byte(data))
	return err
}

func (i *Input) SetOnConfirm(callback EventCallback) {
	i.onConfirm = callback
}

func (i *Input) SetOnLeave(callback EventCallback) {
	i.onLeave = callback
}

func (i *Input) setDefaultKeybindings() {
	i.bindConfirmOnKey(gocui.KeyEnter)
	i.bindConfirmOnKey(gocui.KeyArrowDown)
	i.bindLeaveOnKey(gocui.KeyEsc)
	i.bindLeaveOnKey(gocui.KeyArrowUp)
}

func (i *Input) bindConfirmOnKey(key gocui.Key) error {
	inputViewName := i.Name + "-input"
	return i.g.SetKeybinding(inputViewName, key, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if data, err := i.GetData(); err != nil {
			return err
		} else {
			if i.onConfirm != nil {
				if err := i.onConfirm(data, key); err != nil {
					return err
				}
			}
			return nil
		}
	})
}

func (i *Input) bindLeaveOnKey(key gocui.Key) error {
	inputViewName := i.Name + "-input"
	return i.g.SetKeybinding(inputViewName, key, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if data, err := i.GetData(); err != nil {
			return err
		} else {
			if i.onConfirm != nil {
				if err := i.onLeave(data, key); err != nil {
					return err
				}
			}
			return nil
		}
	})
}
