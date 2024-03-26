package console

import (
	"github.com/jroimartin/gocui"

	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/harvester/harvester-installer/pkg/widgets"
)

type passwordWrapper struct {
	c                *Console
	passwordV        *widgets.Input
	passwordConfirmV *widgets.Input
}

func (p *passwordWrapper) passwordVConfirmKeyBinding(_ *gocui.Gui, _ *gocui.View) error {
	password1V, err := p.c.GetElement(passwordPanel)
	if err != nil {
		return err
	}
	userInputData.Password, err = password1V.GetData()
	if err != nil {
		return err
	}
	if userInputData.Password == "" {
		return p.c.setContentByName(validatorPanel, "Password is required")
	}
	return showNext(p.c, passwordConfirmPanel)
}

func (p *passwordWrapper) passwordVEscapeKeyBinding(_ *gocui.Gui, _ *gocui.View) error {
	p.passwordV.Close()
	p.passwordConfirmV.Close()
	if installModeOnly {
		return showDiskPage(p.c)
	}
	if err := p.c.setContentByName(notePanel, ""); err != nil {
		return err
	}
	return showNext(p.c, tokenPanel)
}

func (p *passwordWrapper) passwordConfirmVArrowUpKeyBinding(_ *gocui.Gui, _ *gocui.View) error {
	var err error
	userInputData.PasswordConfirm, err = p.passwordConfirmV.GetData()
	if err != nil {
		return err
	}
	return showNext(p.c, passwordPanel)
}

func (p *passwordWrapper) passwordConfirmVKeyEnter(_ *gocui.Gui, _ *gocui.View) error {
	var err error
	userInputData.PasswordConfirm, err = p.passwordConfirmV.GetData()
	if err != nil {
		return err
	}
	if userInputData.Password != userInputData.PasswordConfirm {
		return p.c.setContentByName(validatorPanel, "Password mismatching")
	}
	p.passwordV.Close()
	p.passwordConfirmV.Close()
	encrypted, err := util.GetEncryptedPasswd(userInputData.Password)
	if err != nil {
		return err
	}
	p.c.config.Password = encrypted
	//TODO: When booted in install mode.. show steps for application of config
	if installModeOnly {
		return showNext(p.c, confirmInstallPanel)
	}
	return showNext(p.c, ntpServersPanel)
}

func (p *passwordWrapper) passwordConfirmVKeyEscape(_ *gocui.Gui, _ *gocui.View) error {
	p.passwordV.Close()
	p.passwordConfirmV.Close()
	if err := p.c.setContentByName(notePanel, ""); err != nil {
		return err
	}
	if installModeOnly {
		return showDiskPage(p.c)
	}
	return showNext(p.c, tokenPanel)
}
