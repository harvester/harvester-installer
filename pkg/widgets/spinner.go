package widgets

import (
	"fmt"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
)

type Spinner struct {
	g      *gocui.Gui
	panel  string
	prefix string
	ticker *time.Ticker
	focus  bool

	stop    chan struct{}
	stopped chan bool
}

const (
	infoColor    = gocui.ColorCyan
	errorColor   = gocui.ColorRed
	spinInterval = 100 * time.Millisecond
)

func NewSpinner(g *gocui.Gui, panel string, prefix string) *Spinner {
	return &Spinner{
		g:       g,
		panel:   panel,
		prefix:  prefix,
		stop:    make(chan struct{}),
		stopped: make(chan bool),
	}
}

func NewFocusSpinner(g *gocui.Gui, panel string, prefix string) *Spinner {
	return &Spinner{
		g:       g,
		panel:   panel,
		prefix:  prefix,
		stop:    make(chan struct{}),
		stopped: make(chan bool),
		focus:   true,
	}
}

func (s *Spinner) Start() {
	s.ticker = time.NewTicker(spinInterval)
	go func() {
		s.writePanel(fmt.Sprintf("%s|", s.prefix), true, infoColor)
		for {
			for _, symbol := range `\-/|` {
				select {
				case <-s.stop:
					return
				case <-s.ticker.C:
					s.writePanel(fmt.Sprintf("\r%s%c", s.prefix, symbol), false, infoColor)
				}
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.ticker.Stop()
	s.stop <- struct{}{}
}

func (s *Spinner) writePanel(message string, clear bool, fgColor gocui.Attribute) {
	// g.Update spawns a goroutine to notify gocui to update.
	// wait until cui consumes the notification to make sure any remaining
	// writePanel calls don't go first
	ch := make(chan struct{})

	s.g.Update(func(g *gocui.Gui) error {
		defer func() {
			ch <- struct{}{}
		}()
		v, err := g.View(s.panel)
		if err == gocui.ErrUnknownView {
			return nil
		}
		if err != nil {
			return err
		}

		if s.focus {
			logrus.Info("SetCurrentView", s.panel)
			g.SetCurrentView(s.panel)
		}
		if clear {
			v.Clear()
		}
		v.FgColor = fgColor
		v.Write([]byte(message))
		return nil
	})

	<-ch
}
