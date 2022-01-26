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
	GetNote() string
	GetFooterNote() string
	// TODO: Generalize the SetData interface:
	// SetData(interface{}) error
}

func createVerticalLocator(g *gocui.Gui) func(component PageComponent, height int) {
	maxX, maxY := g.Size()
	lastY := 0
	return func(component PageComponent, height int) {
		if component == nil {
			lastY += height
			return
		}
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
	note          *Panel
	status        *Panel
	footer        *Panel
	components    []PageComponent
	statusSpinner *Spinner
	currentFocus  PageComponent

	// callbacks
	onNextPage EventCallback
	onPrevPage EventCallback
}

func NewPage(g *gocui.Gui, name string) (*Page, error) {
	title := NewPanel(g, fmt.Sprintf("%s-title", name))
	title.Focus = false
	note := NewPanel(g, fmt.Sprintf("%s-note", name))
	note.Focus = false
	status := NewPanel(g, fmt.Sprintf("%s-status", name))
	status.Focus = false
	footer := NewPanel(g, fmt.Sprintf("%s-footer", name))
	footer.Focus = false

	page := &Page{
		g:          g,
		title:      title,
		note:       note,
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
	if err := p.note.Show(); err != nil {
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
	logrus.Info("Close page")
	defer logrus.Info("Close page done")
	if err := p.title.Close(); err != nil {
		return err
	}
	if err := p.note.Show(); err != nil {
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

func (p *Page) SetNote(msg string) {
	p.note.SetContent(msg)
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
			if err == gocui.ErrUnknownView {
				return nil
			} else if err != nil {
				return err
			}

			if isError {
				statusView.FgColor = errorColor
			} else {
				statusView.FgColor = normalColor
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
				if editable, ok := targetComp.(EditablePageComponent); ok {
					p.SetFooter(editable.GetFooterNote())
					p.SetNote(editable.GetNote())
				}
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
					logrus.Info("Go to next page")
					p.g.Update(func(g *gocui.Gui) error {
						return p.gotoNextPage(key)
					})
				}
			}
		}()
		return nil
	})

	comp.SetOnLeave(func(data interface{}, key gocui.Key) error {
		switch key {
		case gocui.KeyEsc:
			logrus.Info("Go to prev page")
			return p.gotoPrevPage(key)
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

func (p *Page) SetOnNextPage(callback EventCallback) {
	p.onNextPage = callback
}

func (p *Page) SetOnPrevPage(callback EventCallback) {
	p.onPrevPage = callback
}

// Dummy method for Element interface
func (p *Page) GetData() (string, error) {
	return "", nil
}

// Dummy method for Element interface
func (p *Page) SetContent(string) {
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
	maxX, maxY := p.g.Size()
	p.title.SetLocation(maxX/8, maxY/8-3, maxX/8*7, maxY/8)

	setLocation := createVerticalLocator(p.g)
	setLocation(nil, maxY/8)
	for _, comp := range p.components {
		setLocation(comp, 3)
	}
	setLocation(p.note, 5)
	setLocation(p.status, 3)

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

func (p *Page) gotoNextPage(key gocui.Key) error {
	if p.onNextPage != nil {
		if err := p.Close(); err != nil {
			return err
		}
		return p.onNextPage(nil, key)
	}
	return nil
}

func (p *Page) gotoPrevPage(key gocui.Key) error {
	if p.onPrevPage != nil {
		if err := p.Close(); err != nil {
			return err
		}
		return p.onPrevPage(nil, key)
	}
	return nil
}
