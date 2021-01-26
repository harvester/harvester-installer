package console

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"github.com/jroimartin/gocui"
	cfg "github.com/rancher/harvester-installer/pkg/config"
	"github.com/rancher/harvester-installer/pkg/util"
	"github.com/rancher/harvester-installer/pkg/widgets"
	"github.com/rancher/k3os/pkg/config"
	"github.com/sirupsen/logrus"
)

var (
	once sync.Once
)

func (c *Console) layoutInstall(g *gocui.Gui) error {
	var err error
	once.Do(func() {
		setPanels(c)
		initElements := []string{
			titlePanel,
			validatorPanel,
			notePanel,
			footerPanel,
			askCreatePanel,
		}
		var e widgets.Element
		for _, name := range initElements {
			e, err = c.GetElement(name)
			if err != nil {
				return
			}
			if err = e.Show(); err != nil {
				return
			}
		}
	})
	return err
}

func setPanels(c *Console) error {
	funcs := []func(*Console) error{
		addTitlePanel,
		addValidatorPanel,
		addNotePanel,
		addFooterPanel,
		addDiskPanel,
		addAskCreatePanel,
		addServerURLPanel,
		addPasswordPanels,
		addSSHKeyPanel,
		addNetworkPanel,
		addTokenPanel,
		addProxyPanel,
		addCloudInitPanel,
		addConfirmPanel,
		addInstallPanel,
	}
	for _, f := range funcs {
		if err := f(c); err != nil {
			return err
		}
	}
	return nil
}

func addTitlePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	titleV := widgets.NewPanel(c.Gui, titlePanel)
	titleV.SetLocation(maxX/4, maxY/4-3, maxX/4*3, maxY/4)
	titleV.Focus = false
	c.AddElement(titlePanel, titleV)
	return nil
}

func addValidatorPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	validatorV := widgets.NewPanel(c.Gui, validatorPanel)
	validatorV.SetLocation(maxX/4, maxY/4+5, maxX/4*3, maxY/4+7)
	validatorV.FgColor = gocui.ColorRed
	validatorV.Focus = false
	c.AddElement(validatorPanel, validatorV)
	return nil
}

func addNotePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	noteV := widgets.NewPanel(c.Gui, notePanel)
	noteV.SetLocation(maxX/4, maxY/4+3, maxX, maxY/4+5)
	noteV.Wrap = true
	noteV.Focus = false
	c.AddElement(notePanel, noteV)
	return nil
}

func addFooterPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	footerV := widgets.NewPanel(c.Gui, footerPanel)
	footerV.SetLocation(0, maxY-2, maxX, maxY)
	footerV.Focus = false
	c.AddElement(footerPanel, footerV)
	return nil
}

func addDiskPanel(c *Console) error {
	diskV, err := widgets.NewSelect(c.Gui, diskPanel, "", getDiskOptions)
	if err != nil {
		return err
	}
	diskV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			device, err := diskV.GetData()
			if err != nil {
				return err
			}
			cfg.Config.K3OS.Install = &config.Install{
				Device: device,
			}
			diskV.Close()
			if cfg.Config.InstallMode == modeCreate {
				return showNext(c, tokenPanel)
			}
			return showNext(c, serverURLPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			diskV.Close()
			return showNext(c, askCreatePanel)
		},
	}
	diskV.PreShow = func() error {
		return c.setContentByName(titlePanel, "Choose installation target. Device will be formatted")
	}
	c.AddElement(diskPanel, diskV)
	return nil
}

func getDiskOptions() ([]widgets.Option, error) {
	output, err := exec.Command("/bin/sh", "-c", `lsblk -r -o NAME,SIZE,TYPE | grep -w disk|cut -d ' ' -f 1,2`).CombinedOutput()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	var options []widgets.Option
	for _, line := range lines {
		splits := strings.SplitN(line, " ", 2)
		if len(splits) == 2 {
			options = append(options, widgets.Option{
				Value: "/dev/" + splits[0],
				Text:  line,
			})
		}
	}

	return options, nil
}

func addAskCreatePanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		return []widgets.Option{
			{
				Value: modeCreate,
				Text:  "Create a new Harvester cluster",
			}, {
				Value: modeJoin,
				Text:  "Join an existing Harvester cluster",
			},
		}, nil
	}
	// new cluster or join existing cluster
	askCreateV, err := widgets.NewSelect(c.Gui, askCreatePanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	askCreateV.PreShow = func() error {
		if err := c.setContentByName(footerPanel, ""); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Choose installation mode")
	}
	askCreateV.PostClose = func() error {
		return c.setContentByName(footerPanel, "<Use ESC to go back to previous section>")
	}
	askCreateV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			selected, err := askCreateV.GetData()
			if err != nil {
				return err
			}
			askCreateV.Close()
			if selected == modeCreate {
				cfg.Config.InstallMode = modeCreate
			} else {
				cfg.Config.InstallMode = modeJoin
			}
			return showNext(c, diskPanel)
		},
	}
	c.AddElement(askCreatePanel, askCreateV)
	return nil
}

func addServerURLPanel(c *Console) error {
	serverURLV, err := widgets.NewInput(c.Gui, serverURLPanel, "Management address", false)
	if err != nil {
		return err
	}
	serverURLV.PreShow = func() error {
		c.Gui.Cursor = true
		if err := c.setContentByName(titlePanel, "Configure management address"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, serverURLNote)
	}
	serverURLV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			serverURL, err := serverURLV.GetData()
			if err != nil {
				return err
			}
			if serverURL == "" {
				return c.setContentByName(validatorPanel, "Management address is required")
			}
			serverURLV.Close()
			cfg.Config.K3OS.ServerURL = getFormattedServerURL(serverURL)
			return showNext(c, tokenPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			g.Cursor = false
			serverURLV.Close()
			return showNext(c, diskPanel)
		},
	}
	c.AddElement(serverURLPanel, serverURLV)
	return nil
}

func addPasswordPanels(c *Console) error {
	maxX, maxY := c.Gui.Size()
	passwordV, err := widgets.NewInput(c.Gui, passwordPanel, "Password", true)
	if err != nil {
		return err
	}
	passwordConfirmV, err := widgets.NewInput(c.Gui, passwordConfirmPanel, "Confirm password", true)
	if err != nil {
		return err
	}

	passwordV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			return showNext(c, passwordConfirmPanel)
		},
		gocui.KeyArrowDown: func(g *gocui.Gui, v *gocui.View) error {
			return showNext(c, passwordConfirmPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			passwordV.Close()
			passwordConfirmV.Close()
			if err := c.setContentByName(notePanel, ""); err != nil {
				return err
			}
			return showNext(c, tokenPanel)
		},
	}
	passwordV.SetLocation(maxX/4, maxY/4, maxX/4*3, maxY/4+2)
	c.AddElement(passwordPanel, passwordV)

	passwordConfirmV.PreShow = func() error {
		c.Gui.Cursor = true
		c.setContentByName(notePanel, "")
		return c.setContentByName(titlePanel, "Configure the password to access the node")
	}
	passwordConfirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: func(g *gocui.Gui, v *gocui.View) error {
			return showNext(c, passwordPanel)
		},
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			password1V, err := c.GetElement(passwordPanel)
			if err != nil {
				return err
			}
			password1, err := password1V.GetData()
			if err != nil {
				return err
			}
			password2, err := passwordConfirmV.GetData()
			if err != nil {
				return err
			}
			if password1 != password2 {
				return c.setContentByName(validatorPanel, "Password mismatching")
			}
			if password1 == "" {
				return c.setContentByName(validatorPanel, "Password is required")
			}
			password1V.Close()
			passwordConfirmV.Close()
			encrpyted, err := util.GetEncrptedPasswd(password1)
			if err != nil {
				return err
			}
			cfg.Config.K3OS.Password = encrpyted
			return showNext(c, sshKeyPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			passwordV.Close()
			passwordConfirmV.Close()
			if err := c.setContentByName(notePanel, ""); err != nil {
				return err
			}
			return showNext(c, tokenPanel)
		},
	}
	passwordConfirmV.SetLocation(maxX/4, maxY/4+3, maxX/4*3, maxY/4+5)
	c.AddElement(passwordConfirmPanel, passwordConfirmV)

	return nil
}

func addSSHKeyPanel(c *Console) error {
	sshKeyV, err := widgets.NewInput(c.Gui, sshKeyPanel, "HTTP URL", false)
	if err != nil {
		return err
	}
	sshKeyV.PreShow = func() error {
		c.Gui.Cursor = true
		if err := c.setContentByName(titlePanel, "Optional: import SSH keys"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, "For example: https://github.com/<username>.keys")
	}
	sshKeyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			url, err := sshKeyV.GetData()
			if err != nil {
				return err
			}
			if url != "" {
				pubKeys, err := getRemoteSSHKeys(url)
				if err != nil {
					logrus.Error(err)
					return c.setContentByName(validatorPanel, err.Error())
				}
				logrus.Debug("SSH public keys: ", pubKeys)
				cfg.Config.SSHAuthorizedKeys = pubKeys
			}
			sshKeyV.Close()
			return showNext(c, networkPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			sshKeyV.Close()
			return showNext(c, passwordConfirmPanel, passwordPanel)
		},
	}
	c.AddElement(sshKeyPanel, sshKeyV)
	return nil
}

func addTokenPanel(c *Console) error {
	tokenV, err := widgets.NewInput(c.Gui, tokenPanel, "Cluster token", false)
	if err != nil {
		return err
	}
	tokenV.PreShow = func() error {
		c.Gui.Cursor = true
		if cfg.Config.InstallMode == modeCreate {
			if err := c.setContentByName(notePanel, clusterTokenNote); err != nil {
				return err
			}
		}
		return c.setContentByName(titlePanel, "Configure cluster token")
	}
	tokenV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			token, err := tokenV.GetData()
			if err != nil {
				return err
			}
			if token == "" {
				return c.setContentByName(validatorPanel, "Cluster token is required")
			}
			cfg.Config.K3OS.Token = token
			tokenV.Close()
			return showNext(c, passwordConfirmPanel, passwordPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			tokenV.Close()
			if cfg.Config.InstallMode == modeCreate {
				g.Cursor = false
				return showNext(c, diskPanel)
			}
			return showNext(c, serverURLPanel)
		},
	}
	c.AddElement(tokenPanel, tokenV)
	return nil
}

func addNetworkPanel(c *Console) error {
	networkV, err := widgets.NewSelect(c.Gui, networkPanel, "", getNetworkInterfaceOptions)
	if err != nil {
		return err
	}
	networkV.PreShow = func() error {
		c.Gui.Cursor = false
		return c.setContentByName(titlePanel, "Select interface for the management network")
	}
	networkV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			iface, err := networkV.GetData()
			if err != nil {
				return err
			}
			if iface != "" {
				cfg.Config.ExtraK3sArgs = append(cfg.Config.ExtraK3sArgs, "--flannel-iface", iface)
			}
			networkV.Close()
			return showNext(c, proxyPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			networkV.Close()
			return showNext(c, sshKeyPanel)
		},
	}
	c.AddElement(networkPanel, networkV)
	return nil
}

func getNetworkInterfaceOptions() ([]widgets.Option, error) {
	var options = []widgets.Option{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		var ips []string
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.String())
				}
			}
		}
		option := widgets.Option{
			Value: i.Name,
			Text:  i.Name,
		}
		if len(ips) > 0 {
			option.Text = fmt.Sprintf("%s (%s)", i.Name, strings.Join(ips, ","))
		}
		options = append(options, option)
	}
	return options, nil
}

func addProxyPanel(c *Console) error {
	proxyV, err := widgets.NewInput(c.Gui, proxyPanel, "Proxy address", false)
	if err != nil {
		return err
	}
	proxyV.PreShow = func() error {
		c.Gui.Cursor = true
		if err := c.setContentByName(titlePanel, "Optional: configure proxy"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, proxyNote)
	}
	proxyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			proxy, err := proxyV.GetData()
			if err != nil {
				return err
			}
			if proxy != "" {
				if cfg.Config.K3OS.Environment == nil {
					cfg.Config.K3OS.Environment = make(map[string]string)
				}
				cfg.Config.K3OS.Environment["http_proxy"] = proxy
				cfg.Config.K3OS.Environment["https_proxy"] = proxy
			}
			proxyV.Close()
			noteV, err := c.GetElement(notePanel)
			if err != nil {
				return err
			}
			noteV.Close()
			return showNext(c, cloudInitPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			proxyV.Close()
			return showNext(c, networkPanel)
		},
	}
	c.AddElement(proxyPanel, proxyV)
	return nil
}

func addCloudInitPanel(c *Console) error {
	cloudInitV, err := widgets.NewInput(c.Gui, cloudInitPanel, "HTTP URL", false)
	if err != nil {
		return err
	}
	cloudInitV.PreShow = func() error {
		return c.setContentByName(titlePanel, "Optional: configure cloud-init")
	}
	cloudInitV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			configURL, err := cloudInitV.GetData()
			if err != nil {
				return err
			}
			confirmV, err := c.GetElement(confirmPanel)
			if err != nil {
				return err
			}
			cfg.Config.K3OS.Install.ConfigURL = configURL
			cloudInitV.Close()
			installBytes, err := config.PrintInstall(cfg.Config.CloudConfig)
			if err != nil {
				return err
			}
			options := fmt.Sprintf("install mode: %v\n", cfg.Config.InstallMode)
			if proxy, ok := cfg.Config.K3OS.Environment["http_proxy"]; ok {
				options += fmt.Sprintf("proxy address: %v\n", proxy)
			}
			options += string(installBytes)
			logrus.Debug("cfm cfg: ", fmt.Sprintf("%+v", cfg.Config.K3OS.Install))
			if cfg.Config.K3OS.Install != nil && !cfg.Config.K3OS.Install.Silent {
				confirmV.SetContent(options +
					"\nYour disk will be formatted and Harvester will be installed with \nthe above configuration. Continue?\n")
			}
			g.Cursor = false
			return showNext(c, confirmPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			cloudInitV.Close()
			return showNext(c, proxyPanel)
		},
	}
	c.AddElement(cloudInitPanel, cloudInitV)
	return nil
}

func addConfirmPanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		return []widgets.Option{
			{
				Value: "yes",
				Text:  "Yes",
			}, {
				Value: "no",
				Text:  "No",
			},
		}, nil
	}
	confirmV, err := widgets.NewSelect(c.Gui, confirmPanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	confirmV.PreShow = func() error {
		return c.setContentByName(titlePanel, "Confirm installation options")
	}
	confirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			confirmed, err := confirmV.GetData()
			if err != nil {
				return err
			}
			if confirmed == "no" {
				confirmV.Close()
				c.setContentByName(titlePanel, "")
				go util.SleepAndReboot()
				return c.setContentByName(notePanel, "Installation halted. Rebooting system in 5 seconds")
			}
			confirmV.Close()
			customizeConfig()
			return showNext(c, installPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			confirmV.Close()
			return showNext(c, cloudInitPanel)
		},
	}
	c.AddElement(confirmPanel, confirmV)
	return nil
}

func addInstallPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	installV := widgets.NewPanel(c.Gui, installPanel)
	installV.PreShow = func() error {
		go doInstall(c.Gui)
		return c.setContentByName(footerPanel, "")
	}
	installV.Title = " Installing Harvester "
	installV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8*7)
	c.AddElement(installPanel, installV)
	installV.Frame = true
	return nil
}
