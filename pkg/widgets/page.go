package widgets

import (
	"fmt"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
)

type EventCallback func(data interface{}, key gocui.Key) error

type ComponentValidator func(data interface{}) error

type PageComponent interface {
	Element
	SetLocation(int, int, int, int)
}

type EditablePageComponent interface {
	PageComponent
	SetOnConfirm(EventCallback)
	SetOnLeave(EventCallback)
	// TODO: Generalize the SetData interface:
	// SetData(interface{}) error
}

func createVerticalLocator(g *gocui.Gui) func(component PageComponent, height int) {
	maxX, maxY := g.Size()
	lastY := 0
	return func(component PageComponent, height int) {
		var (
			x0 = maxX / 8
			y0 = lastY
			x1 = maxX / 8 * 7
			y1 int
		)
		if height == 0 {
			y1 = maxY / 8 * 7
		} else {
			y1 = y0 + height
		}
		lastY += height
		component.SetLocation(x0, y0, x1, y1)
	}
}

type Page struct {
	g             *gocui.Gui
	title         *Panel
	status        *Panel
	footer        *Panel
	components    []PageComponent
	statusSpinner *Spinner
	currentFocus  PageComponent
}

func NewPage(g *gocui.Gui, name string) (*Page, error) {
	title := NewPanel(g, fmt.Sprintf("%s-title", name))
	title.Focus = false
	status := NewPanel(g, fmt.Sprintf("%s-status", name))
	status.Focus = false
	footer := NewPanel(g, fmt.Sprintf("%s-footer", name))
	footer.Focus = false

	page := &Page{
		g:          g,
		title:      title,
		status:     status,
		footer:     footer,
		components: []PageComponent{},
	}
	page.layout()

	return page, nil
}

func (p *Page) Show() error {
	if err := p.title.Show(); err != nil {
		return err
	}
	if err := p.status.Show(); err != nil {
		return err
	}
	if err := p.footer.Show(); err != nil {
		return err
	}
	for _, comp := range p.components {
		if err := comp.Show(); err != nil {
			return err
		}
	}

	if p.currentFocus != nil {
		p.SetFocus(p.currentFocus)
	}

	if len(p.components) > 0 {
		p.SetFocus(p.components[0])
	}

	return nil
}

func (p *Page) Close() error {
	if err := p.title.Close(); err != nil {
		return err
	}
	if err := p.status.Close(); err != nil {
		return err
	}
	if err := p.footer.Close(); err != nil {
		return err
	}
	for _, panel := range p.components {
		if err := panel.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) SetTitle(msg string) {
	p.title.SetContent(msg)
}

func (p *Page) SetStatus(msg string, busy, isError bool) {
	if p.statusSpinner != nil {
		p.statusSpinner.Stop()
		p.statusSpinner = nil
	}

	if busy {
		p.statusSpinner = NewFocusSpinner(p.g, p.status.Name, msg)
		p.statusSpinner.Start()
	} else {
		p.g.Update(func(g *gocui.Gui) error {
			statusView, err := p.g.View(p.status.Name)
			if err != nil {
				return err
			}
			if isError {
				statusView.FgColor = errorColor
			} else {
				statusView.FgColor = infoColor
			}
			p.status.SetContent(msg)
			return nil
		})
	}
}

func (p *Page) SetFooter(msg string) {
	p.footer.SetContent(msg)
}

func (p *Page) SetFocus(targetComp PageComponent) {
	p.g.Update(func(g *gocui.Gui) error {
		for _, comp := range p.components {
			if comp == targetComp {
				p.currentFocus = targetComp
				logrus.Info("Focus on:", targetComp)
				return targetComp.Show()
			}
		}
		return fmt.Errorf("unable to focus on %v: not found", targetComp)
	})
}

func (p *Page) AddComponent(comp EditablePageComponent, validator ComponentValidator) error {
	p.components = append(p.components, comp)

	comp.SetOnConfirm(func(data interface{}, key gocui.Key) error {
		validateDone := make(chan error)
		go func() {
			validateDone <- validator(data)
		}()

		go func() {
			err := <-validateDone
			if err != nil {
				logrus.Info("Set err")
				p.SetStatus(err.Error(), false, true)
				p.SetFocus(comp)
			} else {
				logrus.Info("Set noterr")
				p.SetStatus("", false, false)
				if nextComp, ok := p.getNextComponent(comp); ok {
					logrus.Info("Go to next panel")
					logrus.Info(nextComp)
					p.SetFocus(nextComp)
				} else {
					logrus.Info("No next comp")
				}
			}
		}()
		return nil
	})

	comp.SetOnLeave(func(data interface{}, key gocui.Key) error {
		switch key {
		case gocui.KeyEsc:
			logrus.Info("Go to prev page")
		case gocui.KeyArrowUp:
			if prevComp, ok := p.getPrevComponent(comp); ok {
				logrus.Info("Go to prev panel")
				p.SetFocus(prevComp)
			} else {
				logrus.Info("No prev comp")
			}
		default:
			logrus.Warning("Undefined Leave key")
		}
		return nil
	})
	p.layout()
	return nil
}

// TODO: FIX ME
func (p *Page) RemoveComponent(compToRemove PageComponent) error {
	for idx, comp := range p.components {
		if comp == compToRemove {
			if err := comp.Close(); err != nil {
				return err
			}
			p.components = append(p.components[:idx], p.components[idx+1:]...)
			p.layout()
			return nil
		}
	}

	return fmt.Errorf("unable to locate component")
}

func (p *Page) layout() {
	setLocation := createVerticalLocator(p.g)
	setLocation(p.title, 3)
	for _, comp := range p.components {
		setLocation(comp, 5)
	}
	setLocation(p.status, 5)

	maxX, maxY := p.g.Size()
	p.footer.SetLocation(0, maxY-2, maxX, maxY)
	p.g.Cursor = true
}

func (p *Page) getNextComponent(currComp PageComponent) (PageComponent, bool) {
	lastCompIdx := len(p.components) - 1
	for idx, comp := range p.components {
		if comp == currComp && idx < lastCompIdx {
			return p.components[idx+1], true
		}
	}

	return nil, false
}

func (p *Page) getPrevComponent(currComp PageComponent) (PageComponent, bool) {
	for idx, comp := range p.components {
		if comp == currComp && idx != 0 {
			return p.components[idx-1], true
		}
	}

	return nil, false
}
