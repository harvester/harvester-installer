package widgets

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
)

type Option struct {
	Value string
	Text  string
}

type GetOptionsFunc func() ([]Option, error)

type Select struct {
	*Panel
	Value          string
	getOptionsFunc GetOptionsFunc
	options        []Option
	optionV        *gocui.View

	// For multiselect
	values          []string
	multi           bool
	selectedIndexes []bool

	// Event callback
	onConfirm EventCallback
	onLeave   EventCallback
}

func NewSelect(g *gocui.Gui, name string, text string, getOptionsFunc GetOptionsFunc) (*Select, error) {
	return &Select{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: text,
		},
		getOptionsFunc: getOptionsFunc,
	}, nil
}

func (s *Select) Show() error {
	var err error
	if err := s.Panel.Show(); err != nil {
		return err
	}
	if s.getOptionsFunc != nil {
		if s.options, err = s.getOptionsFunc(); err != nil {
			return err
		}
	}
	optionViewName := s.Name + "-options"
	offset := len(strings.Split(s.Content, "\n"))
	y0 := s.Y0 + offset - 1
	y1 := s.Y0 + offset + len(s.options)
	v, err := s.g.SetView(optionViewName, s.X0, y0, s.X1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// Initialize
		v.Wrap = true

		if s.multi {
			// Intialize multiselect view
			if len(s.selectedIndexes) == 0 {
				s.selectedIndexes = make([]bool, len(s.options))
			}
			s.updateSelectedStatus(v)
		} else {
			v.Highlight = true
			v.SelBgColor = gocui.ColorGreen
			v.SelFgColor = gocui.ColorBlack
			for _, opt := range s.options {
				if _, err := fmt.Fprintln(v, opt.Text); err != nil {
					return err
				}
			}
		}

		if err := s.setOptionsKeyBindings(optionViewName); err != nil {
			return err
		}
		/* Don't allow custom keybindings for now
		if s.KeyBindings != nil {
			for key, f := range s.KeyBindings {
				if err := s.g.SetKeybinding(optionViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
		*/
	}

	// If Show() is called, focus this view no matter what
	if _, err := s.g.SetCurrentView(optionViewName); err != nil {
		return err
	}

	if s.multi {
		return nil
	}
	return s.SetData(s.Value)
}

func (s *Select) Close() error {
	optionViewName := s.Name + "-options"
	s.g.DeleteKeybindings(optionViewName)
	if err := s.g.DeleteView(optionViewName); err != nil {
		return err
	}
	return s.Panel.Close()
}

func (s *Select) SetMulti(multi bool) {
	s.multi = multi

	if multi {
		if len(s.KeyBindingTips) == 0 {
			s.KeyBindingTips = map[string]string{}
		}
		s.KeyBindingTips["SPACE"] = "select options"
	} else {
		delete(s.KeyBindingTips, "SPACE")
	}
}

func (s *Select) getOptUnderCursor() (int, Option, error) {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		return 0, Option{}, err
	}
	if len(ov.BufferLines()) == 0 {
		return 0, Option{}, fmt.Errorf("no options")
	}
	_, cy := ov.Cursor()
	var value Option
	if len(s.options) >= cy+1 {
		value = s.options[cy]
	}
	return cy, value, nil
}

func (s *Select) GetData() (string, error) {
	return s.Value, nil
}

func (s *Select) GetMultiData() []string {
	return s.values
}

func (s *Select) SetData(data string) error {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// View is not created, only set Value, don't update buffer
		s.Value = data
		return nil
	}

	// If the given data is not found, we consider it as "nothing selected",
	// and Value would be an empty string
	ox, oy := ov.Origin()
	for i, option := range s.options {
		if option.Value == data {
			if err = ov.SetCursor(ox, oy+i); err != nil {
				return err
			}
			s.Value = data
			break
		}
	}

	return nil
}

/* TODO: Support set multi-select data
func (s *Select) SetMultiData(data string) error {
}
*/

func (s *Select) getDataGeneric() (interface{}, error) {
	var data interface{}
	var err error

	if s.multi {
		return s.GetMultiData(), nil
	} else {
		if data, err = s.GetData(); err != nil {
			return nil, err
		} else {
			return data, err
		}
	}
}

func (s *Select) SetOnConfirm(callback EventCallback) {
	s.onConfirm = callback
}

func (s *Select) SetOnLeave(callback EventCallback) {
	s.onLeave = callback
}

func (s *Select) updateSelectedStatus(v *gocui.View) error {
	v.Clear()
	_, cy := v.Cursor()
	v.SetCursor(1, cy)
	values := make([]string, 0)
	for i, opt := range s.options {
		selected := " "
		if s.selectedIndexes[i] {
			selected = "x"
			values = append(values, opt.Value)
		}
		if _, err := fmt.Fprintf(v, "[%s] %s\n", selected, opt.Text); err != nil {
			return err
		}
	}
	s.values = values
	s.Value = strings.Join(values, ",")
	return nil
}

func (s *Select) setOptionsKeyBindings(viewName string) error {
	// Update value
	// TODO: Generic handler for both multi and not-multi
	if s.multi {
		handler := func(g *gocui.Gui, v *gocui.View) error {
			optIdx, _, err := s.getOptUnderCursor()
			if err != nil {
				return err
			}
			s.selectedIndexes[optIdx] = !s.selectedIndexes[optIdx]
			s.updateSelectedStatus(v)
			return nil
		}
		if err := s.g.SetKeybinding(viewName, gocui.KeySpace, gocui.ModNone, handler); err != nil {
			return err
		}
	} else {
		handler := func(g *gocui.Gui, v *gocui.View) error {
			_, opt, err := s.getOptUnderCursor()
			if err != nil {
				return err
			}
			s.SetData(opt.Value)
			return nil
		}
		if err := s.g.SetKeybinding(viewName, gocui.KeyEnter, gocui.ModNone, handler); err != nil {
			return err
		}
	}

	// Submit value
	s.g.SetKeybinding(viewName, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		data, err := s.getDataGeneric()
		if err != nil {
			return err
		}

		if s.onConfirm != nil {
			if err := s.onConfirm(data, gocui.KeyEnter); err != nil {
				return err
			}
		}
		return nil
	})
	s.g.SetKeybinding(viewName, gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		data, err := s.getDataGeneric()
		if err != nil {
			return err
		}

		if s.onLeave != nil {
			if err := s.onLeave(data, gocui.KeyEsc); err != nil {
				return err
			}
		}
		return nil
	})
	s.g.SetKeybinding(viewName, gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if v == nil {
			return nil
		}

		if isAtTop(v) {
			data, err := s.getDataGeneric()
			if err != nil {
				return err
			}
			if s.onLeave != nil {
				if err := s.onLeave(data, gocui.KeyArrowUp); err != nil {
					return err
				}
			}
		} else {
			cx, cy := v.Cursor()
			if err := v.SetCursor(cx, cy-1); err != nil {
				ox, oy := v.Origin()
				if err := v.SetOrigin(ox, oy-1); err != nil {
					return err
				}
			}
		}

		return nil
	})
	s.g.SetKeybinding(viewName, gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
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
	})

	return nil
}
