package widgets

import (
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
)

type DropDown struct {
	*Panel
	Select   *Select
	ViewName string
	InputLen int
	Value    string
	Text     string

	// For multiselect dropdown
	multi bool

	// Callbacks
	onConfirm EventCallback
	onLeave   EventCallback
}

func NewDropDown(g *gocui.Gui, name, label string, getOptionsFunc GetOptionsFunc) (*DropDown, error) {
	maxX, maxY := g.Size()
	return &DropDown{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: label,
			X0:      maxX / 8,
			Y0:      maxY / 8,
			X1:      maxX / 8 * 7,
			Y1:      maxY/8 + 3,
			KeyBindingTips: map[string]string{
				"TAB": "choose other options",
			},
		},
		Select: &Select{
			Panel: &Panel{
				Name:        name + "-dropdown-select",
				g:           g,
				KeyBindings: map[gocui.Key]func(*gocui.Gui, *gocui.View) error{},
				Frame:       true,
			},
			getOptionsFunc: getOptionsFunc,
		},
		ViewName: name + "-dropdown",
	}, nil
}

func (d *DropDown) SetMulti(multi bool) {
	d.multi = multi
	d.Select.SetMulti(true)
}

func (d *DropDown) Show() error {
	var err error
	if err = d.Panel.Show(); err != nil {
		return err
	}
	if d.Select.getOptionsFunc != nil {
		if d.Select.options, err = d.Select.getOptionsFunc(); err != nil {
			return err
		}
	}
	offset := 20
	if d.Content == "" {
		offset = 0
	}
	if len(d.Content) > offset {
		offset = len(d.Content) + 1
	}
	x0 := d.X0 + offset
	x1 := d.X1 - 1
	y0 := d.Y0
	y1 := d.Y0 + 2
	d.InputLen = x1 - x0 - 1
	v, err := d.g.SetView(d.ViewName, x0, y0, x1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Wrap = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		if d.Value == "" && d.Text == "" && len(d.Select.options) > 0 {
			if d.multi {
				v.Highlight = false
			} else {
				d.Value = d.Select.options[0].Value
				d.Text = d.Select.options[0].Text
			}
		}
		err = d.g.SetKeybinding(d.ViewName, gocui.KeyTab, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			d.Select.Value = d.Value
			d.Select.Panel.SetLocation(x0, y0, x1, y0+1)
			return d.Select.Show()
		})
		if err != nil {
			return err
		}

		d.Select.SetOnConfirm(func(data interface{}, key gocui.Key) error {
			logrus.Infof("Select confirm: %s, %s", data, d.Select.Value)
			if d.multi {
				// Append multiselect values
				values := data.([]string)
				joined := strings.Join(values, ",")
				d.Value = joined
				d.Text = joined
			} else {
				// TODO: Fix this logic that should be internal to Select
				if len(v.BufferLines()) == 0 {
					return nil
				}
				_, cy := v.Cursor()
				if len(d.Select.options) >= cy+1 {
					d.Value = d.Select.options[cy].Value
					d.Text = d.Select.options[cy].Text
				}
			}
			if err = d.Select.Close(); err != nil {
				return err
			}
			if err = d.Show(); err != nil {
				return err
			}
			if d.onConfirm != nil {
				if err := d.onConfirm(data, key); err != nil {
					return err
				}
			}
			return nil
		})
		d.Select.SetOnLeave(func(data interface{}, key gocui.Key) error {
			logrus.Info("Select Leave", data, key)
			return nil
		})

		if err := d.setDefaultKeybindings(); err != nil {
			return err
		}
	}
	if _, err = d.g.SetCurrentView(d.ViewName); err != nil {
		return err
	}

	return d.SetData(d.Value)
}

func (d *DropDown) Close() error {
	d.g.DeleteKeybindings(d.ViewName)
	if err := d.g.DeleteView(d.ViewName); err != nil {
		return err
	}
	return d.Panel.Close()
}

func (d *DropDown) GetData() (string, error) {
	return d.Value, nil
}

func (s *DropDown) GetMultiData() []string {
	return s.Select.GetMultiData()
}

func (d *DropDown) SetData(data string) error {
	v, err := d.g.View(d.ViewName)
	if err != nil {
		return err
	}
	v.Clear()

	render := func(text string) {
		textLen := len(text)
		if d.InputLen > textLen {
			v.Write([]byte(text))
			for i := 0; i < d.InputLen-textLen-1; i++ {
				v.Write([]byte{' '})
			}
		} else {
			for i := 0; i < d.InputLen-1; i++ {
				v.Write([]byte{text[i]})
			}
		}
		v.Write([]byte{'>'})
	}

	if d.multi {
		// TODO: Fix: data will not be stored for multi-select
		render(d.Value)
	} else {
		for _, option := range d.Select.options {
			if option.Value == data {
				text := option.Text
				render(text)
				break
			}
		}
	}
	return nil
}

func (d *DropDown) SetOnConfirm(callback EventCallback) {
	d.onConfirm = callback
}

func (d *DropDown) SetOnLeave(callback EventCallback) {
	d.onLeave = callback
}

func (d *DropDown) setDefaultKeybindings() error {
	d.g.SetKeybinding(d.ViewName, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		var data interface{}
		var err error

		if d.multi {
			data = d.Select.GetMultiData()
		} else {
			data, err = d.Select.GetData()
			if err != nil {
				return err
			}
		}

		if d.onConfirm != nil {
			if err := d.onConfirm(data, gocui.KeyEnter); err != nil {
				return err
			}
		}
		return nil
	})

	d.g.SetKeybinding(d.ViewName, gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		var data interface{}
		var err error

		if d.multi {
			data = d.Select.GetMultiData()
		} else {
			data, err = d.Select.GetData()
			if err != nil {
				return err
			}
		}

		if d.onConfirm != nil {
			if err := d.onLeave(data, gocui.KeyArrowUp); err != nil {
				return err
			}
		}
		return nil
	})

	d.g.SetKeybinding(d.ViewName, gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		var data interface{}
		var err error

		if d.multi {
			data = d.Select.GetMultiData()
		} else {
			data, err = d.Select.GetData()
			if err != nil {
				return err
			}
		}

		if d.onConfirm != nil {
			if err := d.onLeave(data, gocui.KeyEsc); err != nil {
				return err
			}
		}
		return nil
	})

	return nil
}
