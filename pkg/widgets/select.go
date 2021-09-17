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

		if _, err := s.g.SetCurrentView(optionViewName); err != nil {
			return err
		}

		if err := s.setOptionsKeyBindings(optionViewName); err != nil {
			return err
		}
		if s.KeyBindings != nil {
			for key, f := range s.KeyBindings {
				if err := s.g.SetKeybinding(optionViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
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

func (s *Select) GetData() (string, error) {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		return "", err
	}
	if len(ov.BufferLines()) == 0 {
		return "", nil
	}
	_, cy := ov.Cursor()
	var value string
	if len(s.options) >= cy+1 {
		value = s.options[cy].Value
	}
	return value, nil
}

func (s *Select) GetMultiData() []string {
	return s.values
}

func (s *Select) SetData(data string) error {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		return err
	}
	cx, cy := ov.Cursor()
	for i, option := range s.options {
		if option.Value == data {
			if err = ov.SetCursor(cx, cy+i); err != nil {
				return err
			}
			break
		}
	}
	return nil
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
	setOptionsKeyBindings(s.g, viewName)
	if s.multi {
		handler := func(g *gocui.Gui, v *gocui.View) error {
			_, cy := v.Cursor()
			if len(s.options) >= cy+1 {
				s.selectedIndexes[cy] = !s.selectedIndexes[cy]
			}
			s.updateSelectedStatus(v)
			return nil
		}
		if err := s.g.SetKeybinding(viewName, gocui.KeySpace, gocui.ModNone, handler); err != nil {
			return err
		}
	}
	return nil
}

func setOptionsKeyBindings(g *gocui.Gui, viewName string) error {
	if err := g.SetKeybinding(viewName, gocui.KeyArrowUp, gocui.ModNone, ArrowUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(viewName, gocui.KeyArrowDown, gocui.ModNone, ArrowDown); err != nil {
		return err
	}
	return nil
}
