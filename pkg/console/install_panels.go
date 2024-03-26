package console

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/harvester/harvester-installer/pkg/version"
	"github.com/harvester/harvester-installer/pkg/widgets"
)

type UserInputData struct {
	ServerURL            string
	SSHKeyURL            string
	Password             string
	PasswordConfirm      string
	Address              string
	DNSServers           string
	NTPServers           string
	HasCheckedNTPServers bool
}

const (
	NICStateNotFound = iota
	NICStateDown
	NICStateLowerDown
	NICStateUP
)

const (
	ErrMsgVLANShouldBeANumberInRange string = "VLAN ID should be a number 1 ~ 4094."
	ErrMsgMTUShouldBeANumber         string = "MTU should be a number."
	NtpSettingName                   string = "ntp-servers"
)

var (
	once          sync.Once
	userInputData = UserInputData{
		NTPServers: "0.suse.pool.ntp.org",
	}
	mgmtNetwork = config.Network{
		DefaultRoute: true,
	}
	alreadyInstalled bool
	installModeOnly  bool
	diskConfirmed    bool
)

func (c *Console) layoutInstall(g *gocui.Gui) error {
	var err error
	once.Do(func() {
		setPanels(c)
		initPanel := askCreatePanel

		c.config.OS.Modules = []string{"kvm", "vhost_net"}

		// if already installed then lets check if cloud init allows us to provision
		if alreadyInstalled {
			err = mergeCloudInit(c.config)
			if err != nil {
				logrus.Errorf("error merging cloud-config")
			}
			logrus.Infof("already install value post config merge: %v", c.config.Automatic)
			// if already installed and automatic installation is set to true
			// configure node directly
			if alreadyInstalled && c.config.Automatic {
				initPanel = installPanel
			}
		}

		if cfg, err := config.ReadConfig(); err == nil {
			if cfg.Install.Automatic && isFirstConsoleTTY() {
				logrus.Info("Start automatic installation...")
				c.config.Merge(cfg)
				// setup InstallMode to ensure that during automatic install
				// we are only copying binaries and ignoring network / rancherd setup
				// needed for generating pre-installed qcow2 image
				if c.config.Install.Mode == config.ModeInstall && !alreadyInstalled {
					installModeOnly = true
				}
				initPanel = installPanel
			}
		} else {
			logrus.Errorf("automatic install failed: %v\n", err)
		}

		// add SchemeVersion in non-automatic mode
		// in automatic mode, SchemeVersion should be from config.yaml directly
		if !c.config.Install.Automatic {
			c.config.SchemeVersion = config.SchemeVersion
		}

		initElements := []string{
			titlePanel,
			validatorPanel,
			notePanel,
			footerPanel,
			initPanel,
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
		addAskCreatePanel,
		addDiskPanel,
		addHostnamePanel,
		addNetworkPanel,
		addVIPPanel,
		addDNSServersPanel,
		addNTPServersPanel,
		addServerURLPanel,
		addTokenPanel,
		addPasswordPanels,
		addSSHKeyPanel,
		addProxyPanel,
		addCloudInitPanel,
		addConfirmInstallPanel,
		addConfirmUpgradePanel,
		addInstallPanel,
		addSpinnerPanel,
		addUpgradePanel,
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
	titleV.SetLocation(maxX/8, maxY/8-3, maxX/8*7, maxY/8)
	titleV.Focus = false
	c.AddElement(titlePanel, titleV)
	return nil
}

func addValidatorPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	validatorV := widgets.NewPanel(c.Gui, validatorPanel)
	validatorV.SetLocation(maxX/8, maxY/8+5, maxX/8*7, maxY/8*7)
	validatorV.FgColor = gocui.ColorRed
	validatorV.Wrap = true
	validatorV.Focus = false
	c.AddElement(validatorPanel, validatorV)
	return nil
}

func addNotePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	noteV := widgets.NewPanel(c.Gui, notePanel)
	noteV.SetLocation(maxX/8, maxY/8+3, maxX/8*7, maxY/8+6)
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

func showDiskPage(c *Console) error {
	diskConfirmed = false

	diskOptions, err := getDiskOptions()
	if err != nil {
		return err
	}

	showPersistentSizeOption := false
	if len(diskOptions) == 1 || c.config.Install.DataDisk == c.config.Install.Device {
		showPersistentSizeOption = true
	}

	nextComponents := []string{diskPanel}
	if len(diskOptions) > 1 {
		nextComponents = append([]string{dataDiskPanel}, nextComponents...)
	}

	if systemIsBIOS() {
		nextComponents = append([]string{askForceMBRTitlePanel, askForceMBRPanel}, nextComponents...)
	}

	if showPersistentSizeOption {
		nextComponents = append([]string{persistentSizePanel}, nextComponents...)
	}
	return showNext(c, nextComponents...)
}

func calculateDefaultPersistentSize(dev string) (string, error) {
	bytes, err := util.GetDiskSizeBytes(dev)
	if err != nil {
		return "", err
	}

	defaultBytes := uint64(float64(bytes) * config.DefaultPersistentPercentageNum)
	defaultSize := util.ByteToGi(defaultBytes)
	if defaultSize < config.PersistentSizeMinGiB {
		defaultSize = config.PersistentSizeMinGiB
	}
	return fmt.Sprintf("%dGi", defaultSize), nil
}

func getDataDiskOptions(hvstConfig *config.HarvesterConfig) ([]widgets.Option, error) {
	// Show the OS disk as "Use the installation disk (<Disk Name>)"
	deviceForOS := hvstConfig.Install.Device
	diskOpts, err := getDiskOptions()
	if err != nil {
		return nil, err
	}
	if deviceForOS == "" {
		diskOpts[0].Text = fmt.Sprintf("Use the installation disk (%s)", diskOpts[0].Text)
		return diskOpts, nil
	}

	for i, diskOpt := range diskOpts {
		if diskOpt.Value == deviceForOS {
			osDiskOpt := widgets.Option{
				Text:  fmt.Sprintf("Use the installation disk (%s)", diskOpt.Text),
				Value: diskOpt.Value,
			}
			diskOpts = append(diskOpts[:i], diskOpts[i+1:]...)
			diskOpts = append([]widgets.Option{osDiskOpt}, diskOpts...)
			return diskOpts, nil
		}
	}
	logrus.Warnf("device '%s' not found in disk options", deviceForOS)
	return nil, nil
}

func getDiskOptions() ([]widgets.Option, error) {
	output, err := exec.Command("/bin/sh", "-c", `lsblk -r -o NAME,SIZE,TYPE | grep -w disk | cut -d ' ' -f 1,2`).CombinedOutput()
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

func addDiskPanel(c *Console) error {
	diskConfirmed = false

	setLocation := createVerticalLocator(c)
	diskOpts, err := getDiskOptions()
	if err != nil {
		return err
	}

	// Select device panel
	diskV, err := widgets.NewDropDown(c.Gui, diskPanel, diskLabel, func() ([]widgets.Option, error) {
		return diskOpts, nil
	})
	if err != nil {
		return err
	}
	diskV.PreShow = func() error {
		if c.config.Install.Device == "" {
			c.config.Install.Device = diskOpts[0].Value
		}
		if err := diskV.SetData(c.config.Install.Device); err != nil {
			return err
		}

		if err := c.setContentByName(diskNotePanel, ""); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Choose installation target and data disk. Device will be formatted")
	}
	setLocation(diskV.Panel, 3)
	c.AddElement(diskPanel, diskV)

	dataDiskV, err := widgets.NewDropDown(c.Gui, dataDiskPanel, dataDiskLabel, func() ([]widgets.Option, error) {
		return getDataDiskOptions(c.config)
	})
	if err != nil {
		return err
	}

	dataDiskV.PreShow = func() error {
		if c.config.Install.DataDisk == "" {
			c.config.Install.DataDisk = c.config.Install.Device
		}
		return dataDiskV.SetData(c.config.Install.DataDisk)
	}
	setLocation(dataDiskV.Panel, 3)
	c.AddElement(dataDiskPanel, dataDiskV)

	// Persistent partition size panel
	persistentSizeV, err := widgets.NewInput(c.Gui, persistentSizePanel, persistentSizeLabel, false)
	if err != nil {
		return err
	}
	persistentSizeV.PreShow = func() error {
		c.Cursor = true

		device := c.config.Install.Device
		if device == "" {
			device = diskOpts[0].Value
		}

		//If the user has already set a persistent partition size, use that
		if persistentSizeV.Value != "" {
			if c.config.Install.PersistentPartitionSize != "" {
				persistentSizeV.Value = c.config.Install.PersistentPartitionSize
			} else {
				defaultValue, err := calculateDefaultPersistentSize(device)
				if err != nil {
					return err
				}
				persistentSizeV.Value = defaultValue
			}
		} else {
			defaultValue, err := calculateDefaultPersistentSize(device)
			if err != nil {
				return err
			}
			persistentSizeV.Value = defaultValue
		}
		return nil
	}
	setLocation(persistentSizeV, 3)
	c.AddElement(persistentSizePanel, persistentSizeV)

	// Asking force MBR title
	askForceMBRTitleV := widgets.NewPanel(c.Gui, askForceMBRTitlePanel)
	askForceMBRTitleV.SetContent("Use MBR partitioning scheme")
	setLocation(askForceMBRTitleV, 3)
	c.AddElement(askForceMBRTitlePanel, askForceMBRTitleV)

	// Asking force MBR DropDown
	askForceMBRV, err := widgets.NewDropDown(c.Gui, askForceMBRPanel, "", func() ([]widgets.Option, error) {
		return []widgets.Option{{Value: "no", Text: "No"}, {Value: "yes", Text: "Yes"}}, nil
	})
	if err != nil {
		return err
	}
	askForceMBRV.PreShow = func() error {
		c.Cursor = true
		if c.config.ForceMBR {
			return askForceMBRV.SetData("yes")
		}
		return askForceMBRV.SetData("no")
	}
	setLocation(askForceMBRV.Panel, 3)
	c.AddElement(askForceMBRPanel, askForceMBRV)

	// Note panel for ForceMBR and persistent partition size
	diskNoteV := widgets.NewPanel(c.Gui, diskNotePanel)
	diskNoteV.Wrap = true
	setLocation(diskNoteV, 3)
	c.AddElement(diskNotePanel, diskNoteV)

	// Panel for showing validator message
	diskValidatorV := widgets.NewPanel(c.Gui, diskValidatorPanel)
	diskValidatorV.FgColor = gocui.ColorRed
	diskValidatorV.Wrap = true
	updateValidatorMessage := func(msg string) error {
		diskValidatorV.Focus = false
		return c.setContentByName(diskValidatorPanel, msg)
	}
	setLocation(diskValidatorV, 3)
	c.AddElement(diskValidatorPanel, diskValidatorV)

	// Helper functions
	validateAllDiskSizes := func() (bool, error) {
		installDisk := c.config.Install.Device
		dataDisk := c.config.Install.DataDisk

		if dataDisk == "" || installDisk == dataDisk {
			if err := validateDiskSize(installDisk, true); err != nil {
				return false, updateValidatorMessage(err.Error())
			}
		} else {
			if err := validateDiskSize(installDisk, false); err != nil {
				return false, updateValidatorMessage(err.Error())
			}
			if err := validateDataDiskSize(dataDisk); err != nil {
				return false, updateValidatorMessage(err.Error())
			}
		}
		return true, nil
	}
	closeThisPage := func() {
		c.CloseElements(
			diskPanel,
			dataDiskPanel,
			askForceMBRPanel,
			diskValidatorPanel,
			diskNotePanel,
			askForceMBRTitlePanel,
			persistentSizePanel,
		)
	}
	gotoPrevPage := func(_ *gocui.Gui, _ *gocui.View) error {
		closeThisPage()
		diskConfirmed = false
		return showNext(c, askCreatePanel)
	}
	gotoNextPage := func(_ *gocui.Gui, _ *gocui.View) error {
		// Don't proceed to the next page if disk size validation fails
		if valid, err := validateAllDiskSizes(); !valid || err != nil {
			return err
		}

		installDisk := c.config.Install.Device
		dataDisk := c.config.Install.DataDisk
		persistentSize := c.config.Install.PersistentPartitionSize
		if dataDisk == "" || installDisk == dataDisk {
			diskSize, err := util.GetDiskSizeBytes(c.config.Install.Device)
			if err != nil {
				return err
			}
			if _, err := util.ParsePartitionSize(diskSize, persistentSize); err != nil {
				return updateValidatorMessage(err.Error())
			}
		}

		if !diskConfirmed {
			diskConfirmed = true
			return nil
		}

		closeThisPage()
		//TODO: When Install modeonly.. we need to decide that this
		// network page is not shown and skip straight to the password page
		if installModeOnly {
			return showNext(c, passwordConfirmPanel, passwordPanel)
		}
		return showHostnamePage(c)
	}

	diskConfirm := func(_ *gocui.Gui, _ *gocui.View) error {
		device, err := diskV.GetData()
		if err != nil {
			return err
		}
		dataDisk, err := dataDiskV.GetData()
		if err != nil {
			return err
		}

		if err := updateValidatorMessage(""); err != nil {
			return err
		}
		c.config.Install.Device = device

		if len(diskOpts) > 1 {
			// Show error if disk size validation fails, but allow proceeding to next field
			if _, err := validateAllDiskSizes(); err != nil {
				return err
			}
			if device == dataDisk {
				return showNext(c, persistentSizePanel, dataDiskPanel)
			}
			return showNext(c, dataDiskPanel)
		}

		if err := c.setContentByName(diskNotePanel, persistentSizeNote); err != nil {
			return err
		}
		// Show error if disk size validation fails, but allow proceeding to next field
		if _, err := validateAllDiskSizes(); err != nil {
			return err
		}
		return showNext(c, persistentSizePanel)
	}
	// Keybindings
	diskV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter:     diskConfirm,
		gocui.KeyArrowDown: diskConfirm,
		gocui.KeyArrowUp: func(_ *gocui.Gui, _ *gocui.View) error {
			return updateValidatorMessage("")
		},
		gocui.KeyEsc: gotoPrevPage,
	}

	dataDiskConfirm := func(g *gocui.Gui, v *gocui.View) error {
		dataDisk, err := dataDiskV.GetData()
		if err != nil {
			return err
		}

		if err := updateValidatorMessage(""); err != nil {
			return err
		}
		c.config.Install.DataDisk = dataDisk

		installDisk, err := diskV.GetData()
		if err != nil {
			return err
		}
		if installDisk == dataDisk {
			if err := c.setContentByName(diskNotePanel, persistentSizeNote); err != nil {
				return err
			}
			// Show error if disk size validation fails, but allow proceeding to next field
			if _, err := validateAllDiskSizes(); err != nil {
				return err
			}
			return showNext(c, persistentSizePanel)
		}

		c.CloseElements(persistentSizePanel)
		if systemIsBIOS() {
			if err := c.setContentByName(diskNotePanel, forceMBRNote); err != nil {
				return err
			}
			return showNext(c, askForceMBRPanel)
		}
		return gotoNextPage(g, v)
	}
	dataDiskV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter:     dataDiskConfirm,
		gocui.KeyArrowDown: dataDiskConfirm,
		gocui.KeyArrowUp: func(_ *gocui.Gui, _ *gocui.View) error {
			if err := updateValidatorMessage(""); err != nil {
				return err
			}
			diskConfirmed = false
			return showNext(c, diskPanel)
		},
		gocui.KeyEsc: gotoPrevPage,
	}

	persistentSizeConfirm := func(g *gocui.Gui, v *gocui.View) error {
		persistentSize, err := persistentSizeV.GetData()
		if err != nil {
			return err
		}
		c.config.Install.PersistentPartitionSize = persistentSize

		if systemIsBIOS() {
			if err := c.setContentByName(diskNotePanel, forceMBRNote); err != nil {
				return err
			}
			return showNext(c, askForceMBRPanel)
		}
		return gotoNextPage(g, v)
	}
	persistentSizeV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: persistentSizeConfirm,
		gocui.KeyArrowUp: func(_ *gocui.Gui, _ *gocui.View) error {
			diskConfirmed = false
			if len(diskOpts) > 1 {
				if err := updateValidatorMessage(""); err != nil {
					return err
				}
				if err := c.setContentByName(diskNotePanel, ""); err != nil {
					return err
				}
				return showNext(c, dataDiskPanel)
			}
			if err := updateValidatorMessage(""); err != nil {
				return err
			}
			if err := c.setContentByName(diskNotePanel, ""); err != nil {
				return err
			}
			return showNext(c, diskPanel)
		},
		gocui.KeyArrowDown: persistentSizeConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}

	mbrConfirm := func(g *gocui.Gui, v *gocui.View) error {
		forceMBR, err := askForceMBRV.GetData()
		if err != nil {
			return err
		}
		if forceMBR == "yes" {
			diskTooLargeForMBR, err := diskExceedsMBRLimit(c.config.Device)
			if err != nil {
				return err
			}
			if diskTooLargeForMBR {
				return updateValidatorMessage("Disk too large for MBR. Must be less than 2TiB")
			}
		}

		c.config.ForceMBR = forceMBR == "yes"
		return gotoNextPage(g, v)
	}
	askForceMBRV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: mbrConfirm,
		gocui.KeyArrowUp: func(_ *gocui.Gui, _ *gocui.View) error {
			diskConfirmed = false

			disk, err := diskV.GetData()
			if err != nil {
				return err
			}
			dataDisk, err := dataDiskV.GetData()
			if err != nil {
				return err
			}

			if len(diskOpts) > 1 && disk != dataDisk {
				return showNext(c, dataDiskPanel)
			}
			if err := c.setContentByName(diskNotePanel, persistentSizeNote); err != nil {
				return err
			}
			return showNext(c, persistentSizePanel)
		},
		gocui.KeyArrowDown: mbrConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	return nil
}

func addAskCreatePanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		options := []widgets.Option{
			{
				Value: config.ModeCreate,
				Text:  "Create a new Harvester cluster",
			}, {
				Value: config.ModeJoin,
				Text:  "Join an existing Harvester cluster",
			},
		}
		installed, err := harvesterInstalled()
		if err != nil {
			logrus.Error(err)
		} else if installed {
			options = append(options, widgets.Option{
				Value: config.ModeUpgrade,
				Text:  "Upgrade Harvester",
			})
		}

		// layoutInstall is now called from layoutDashboard due to the addition
		// of the new config.ModeInstall. config will be setup by layoutDashboard before passing control here
		// this extra option should only show up if that is not the case
		if !alreadyInstalled {
			options = append(options, widgets.Option{
				Value: config.ModeInstall,
				Text:  "Install Harvester binaries only",
			})
		}
		return options, nil
	}
	// new cluster or join existing cluster
	askCreateV, err := widgets.NewSelect(c.Gui, askCreatePanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	askCreateV.FirstPage = true
	askCreateV.PreShow = func() error {
		askCreateV.Value = c.config.Install.Mode
		if alreadyInstalled {
			return c.setContentByName(titlePanel, "Harvester already installed. Choose configuration mode")
		}
		return c.setContentByName(titlePanel, "Choose installation mode")
	}
	askCreateV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			selected, err := askCreateV.GetData()
			if err != nil {
				return err
			}

			if alreadyInstalled {
				// need to wipe the Mode value before we set panel
				// needed to ensure that value lookup fails as install
				// is not available in the option in this case
				c.config.Install.Mode = ""
			}

			c.config.Install.Mode = selected
			// explicitly set this false to ensure if user changes from
			// install mode only to create /join then the variable is
			// reset to ensure correct panel sequence is displayed
			installModeOnly = false
			if selected == config.ModeInstall {
				installModeOnly = true
			}

			askCreateV.Close()

			if selected == config.ModeCreate {
				c.config.ServerURL = ""
				userInputData.ServerURL = ""
			} else if selected == config.ModeUpgrade {
				return showNext(c, confirmUpgradePanel)
			}

			// all packages are already install
			// configure hostname and network
			if alreadyInstalled {
				return showHostnamePage(c)
			}
			return showDiskPage(c)
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
		serverURLV.Value = userInputData.ServerURL
		if err := c.setContentByName(titlePanel, "Configure management address"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, serverURLNote)
	}
	serverURLV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			asyncTaskV, err := c.GetElement(spinnerPanel)
			if err != nil {
				return err
			}
			asyncTaskV.Close()

			userInputData.ServerURL, err = serverURLV.GetData()
			if err != nil {
				return err
			}

			if userInputData.ServerURL == "" {
				return c.setContentByName(validatorPanel, "Management address is required")
			}

			fmtServerURL, err := getFormattedServerURL(userInputData.ServerURL)
			if err != nil {
				return c.setContentByName(validatorPanel, err.Error())
			}
			c.CloseElement(validatorPanel)

			// focus on task panel to prevent input
			asyncTaskV.Show()

			pingServerURL := fmtServerURL + "/ping"
			spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", pingServerURL))
			spinner.Start()
			go func(g *gocui.Gui) {
				if err = validatePingServerURL(pingServerURL); err != nil {
					spinner.Stop(true, err.Error())
					g.Update(func(_ *gocui.Gui) error {
						return showNext(c, serverURLPanel)
					})
					return
				}
				spinner.Stop(false, "")
				c.config.ServerURL = fmtServerURL
				g.Update(func(_ *gocui.Gui) error {
					serverURLV.Close()
					return showNext(c, tokenPanel)
				})
			}(c.Gui)
			return nil
		},
		gocui.KeyEsc: func(g *gocui.Gui, _ *gocui.View) error {
			g.Cursor = false
			serverURLV.Close()
			return showNext(c, dnsServersPanel)
		},
	}
	serverURLV.PostClose = func() error {
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
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

	pw := &passwordWrapper{
		c:                c,
		passwordV:        passwordV,
		passwordConfirmV: passwordConfirmV,
	}

	pw.passwordV.PreShow = func() error {
		passwordV.Value = userInputData.Password
		return nil
	}

	pw.passwordV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter:     pw.passwordVConfirmKeyBinding,
		gocui.KeyArrowDown: pw.passwordVConfirmKeyBinding,
		gocui.KeyEsc:       pw.passwordVEscapeKeyBinding,
	}

	pw.passwordV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8+2)
	c.AddElement(passwordPanel, pw.passwordV)

	pw.passwordConfirmV.PreShow = func() error {
		c.Gui.Cursor = true
		passwordConfirmV.Value = userInputData.PasswordConfirm
		c.setContentByName(notePanel, "")
		return c.setContentByName(titlePanel, "Configure the password to access the node")
	}
	pw.passwordConfirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: pw.passwordConfirmVArrowUpKeyBinding,
		gocui.KeyEnter:   pw.passwordConfirmVKeyEnter,
		gocui.KeyEsc:     pw.passwordConfirmVKeyEscape,
	}
	pw.passwordConfirmV.SetLocation(maxX/8, maxY/8+3, maxX/8*7, maxY/8+5)
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
		sshKeyV.Value = userInputData.SSHKeyURL
		if err = c.setContentByName(titlePanel, "Optional: import SSH keys"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, sshKeyNote)
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return sshKeyV.Close()
	}
	gotoNextPage := func() error {
		closeThisPage()
		return showNext(c, cloudInitPanel)
	}
	sshKeyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			url, err := sshKeyV.GetData()
			if err != nil {
				return err
			}
			userInputData.SSHKeyURL = url
			c.config.SSHAuthorizedKeys = []string{}
			if url != "" {
				// focus on task panel to prevent ssh input
				asyncTaskV, err := c.GetElement(spinnerPanel)
				if err != nil {
					return err
				}
				asyncTaskV.Close()
				asyncTaskV.Show()

				spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", url))
				spinner.Start()

				go func(g *gocui.Gui) {
					pubKeys, err := getRemoteSSHKeys(url)
					if err != nil {
						spinner.Stop(true, err.Error())
						g.Update(func(_ *gocui.Gui) error {
							return showNext(c, sshKeyPanel)
						})
						return
					}
					spinner.Stop(false, "")
					logrus.Debug("SSH public keys: ", pubKeys)
					c.config.SSHAuthorizedKeys = pubKeys
					g.Update(func(_ *gocui.Gui) error {
						return gotoNextPage()
					})
				}(c.Gui)
				return nil
			}
			return gotoNextPage()
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			closeThisPage()
			return showNext(c, proxyPanel)
		},
	}
	sshKeyV.PostClose = func() error {
		if err := c.setContentByName(notePanel, ""); err != nil {
			return err
		}
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
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
		tokenV.Value = c.config.Token
		tokenNote := clusterTokenJoinNote
		if c.config.Install.Mode == config.ModeCreate {
			tokenNote = clusterTokenCreateNote
		}
		if err = c.setContentByName(notePanel, tokenNote); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Configure cluster token")
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return tokenV.Close()
	}
	tokenV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			token, err := tokenV.GetData()
			if err != nil {
				return err
			}
			if token == "" {
				return c.setContentByName(validatorPanel, "Cluster token is required")
			}
			if err := checkToken(token); err != nil {
				return c.setContentByName(validatorPanel, err.Error())
			}
			c.config.Token = token
			closeThisPage()
			return showNext(c, passwordConfirmPanel, passwordPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, _ *gocui.View) error {
			closeThisPage()
			if c.config.Install.Mode == config.ModeCreate {
				g.Cursor = false
				return showNext(c, vipTextPanel, vipPanel, askVipMethodPanel)
			}
			return showNext(c, serverURLPanel)
		},
	}
	c.AddElement(tokenPanel, tokenV)
	return nil
}

func showNetworkPage(c *Console) error {
	if mgmtNetwork.Method != config.NetworkMethodStatic {
		return showNext(c, askVlanIDPanel, askBondModePanel, askNetworkMethodPanel, askInterfacePanel)
	}
	return showNext(c, askVlanIDPanel, askBondModePanel, askNetworkMethodPanel, addressPanel, gatewayPanel, mtuPanel, askInterfacePanel)
}

func showHostnamePage(c *Console) error {
	setLocation := createVerticalLocatorWithName(c)

	if err := setLocation(hostnamePanel, 3); err != nil {
		return err
	}

	if err := setLocation(hostnameValidatorPanel, 0); err != nil {
		return err
	}
	return showNext(c, hostnamePanel)
}

func addHostnamePanel(c *Console) error {
	hostnameV, err := widgets.NewInput(c.Gui, hostnamePanel, hostNameLabel, false)
	if err != nil {
		return err
	}

	validatorV := widgets.NewPanel(c.Gui, hostnameValidatorPanel)
	validatorV.FgColor = gocui.ColorRed
	validatorV.Focus = false

	maxX, _ := c.Gui.Size()
	validatorV.X1 = maxX / 8 * 6

	updateValidateMessage := func(message string) error {
		return c.setContentByName(hostnameValidatorPanel, message)
	}

	next := func() error {
		c.CloseElements(hostnamePanel, hostnameValidatorPanel)
		return showNetworkPage(c)
	}

	prev := func(_ *gocui.Gui, _ *gocui.View) error {
		c.CloseElements(hostnamePanel, hostnameValidatorPanel)
		if alreadyInstalled {
			return showNext(c, askCreatePanel)
		}
		return showDiskPage(c)
	}

	validate := func() (string, error) {
		hostname, err := hostnameV.GetData()
		if err != nil {
			return "", err
		}

		if hostname == "" {
			return "Must specify hostname.", nil
		}

		if errs := validation.IsDNS1123Subdomain(hostname); len(errs) > 0 {
			return "Invalid hostname. A lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'.", nil
		}
		c.config.Hostname = hostname
		return "", nil
	}

	hostnameV.PreShow = func() error {
		c.Gui.Cursor = true
		hostnameV.Value = c.config.Hostname
		return c.setContentByName(titlePanel, hostnameTitle)
	}
	hostnameV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: prev,
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			message, err := validate()
			if err != nil {
				return err
			}

			if message != "" {
				return updateValidateMessage(message)
			}
			return next()
		},
		gocui.KeyEsc: prev,
	}

	c.AddElement(hostnamePanel, hostnameV)
	c.AddElement(hostnameValidatorPanel, validatorV)
	return nil
}

func addNetworkPanel(c *Console) error {
	setLocation := createVerticalLocator(c)

	askInterfaceV, err := widgets.NewDropDown(c.Gui, askInterfacePanel, askInterfaceLabel, getNetworkInterfaceOptions)
	if err != nil {
		return err
	}

	askVlanIDV, err := widgets.NewInput(c.Gui, askVlanIDPanel, askVlanIDLabel, false)
	if err != nil {
		return err
	}

	askBondModeV, err := widgets.NewDropDown(c.Gui, askBondModePanel, askBondModeLabel, getBondModeOptions)
	if err != nil {
		return err
	}

	askNetworkMethodV, err := widgets.NewDropDown(c.Gui, askNetworkMethodPanel, askNetworkMethodLabel, getNetworkMethodOptions)
	if err != nil {
		return err
	}

	addressV, err := widgets.NewInput(c.Gui, addressPanel, addressLabel, false)
	if err != nil {
		return err
	}

	gatewayV, err := widgets.NewInput(c.Gui, gatewayPanel, gatewayLabel, false)
	if err != nil {
		return err
	}

	mtuV, err := widgets.NewInput(c.Gui, mtuPanel, mtuLabel, false)
	if err != nil {
		return err
	}

	bondNoteV := widgets.NewPanel(c.Gui, bondNotePanel)

	networkValidatorV := widgets.NewPanel(c.Gui, networkValidatorPanel)

	showBondNote := func() error {
		if err := networkValidatorV.Close(); err != nil {
			return err
		}
		if err := bondNoteV.Close(); err != nil {
			return err
		}
		bondNoteV.Focus = false
		return c.setContentByName(bondNotePanel, bondNote)
	}

	updateValidatorMessage := func(msg string) error {
		if err := networkValidatorV.Close(); err != nil {
			return err
		}
		if err := bondNoteV.Close(); err != nil {
			return err
		}
		networkValidatorV.Focus = false
		return c.setContentByName(networkValidatorPanel, msg)
	}

	gotoNextPanel := func(c *Console, name []string, hooks ...func() (string, error)) func(g *gocui.Gui, v *gocui.View) error {
		return func(_ *gocui.Gui, _ *gocui.View) error {
			c.CloseElement(networkValidatorPanel)
			for _, hook := range hooks {
				msg, err := hook()
				if err != nil {
					return err
				}
				if msg != "" {
					return updateValidatorMessage(msg)
				}
			}
			if err := showBondNote(); err != nil {
				return err
			}
			return showNext(c, name...)
		}
	}

	closeThisPage := func() {
		c.CloseElements(
			askInterfacePanel,
			askVlanIDPanel,
			askBondModePanel,
			askNetworkMethodPanel,
			addressPanel,
			gatewayPanel,
			mtuPanel,
			networkValidatorPanel,
			bondNotePanel,
		)
	}

	setupNetwork := func() ([]byte, error) {
		return applyNetworks(
			mgmtNetwork,
			c.config.Hostname,
		)
	}

	preGotoNextPage := func() (string, error) {
		output, err := setupNetwork()
		if err != nil {
			return fmt.Sprintf("Configure network failed: %s %s", string(output), err), nil
		}
		logrus.Infof("Network configuration is applied: %s", output)

		c.config.ManagementInterface = mgmtNetwork

		if mgmtNetwork.Method == config.NetworkMethodDHCP {
			addr, err := getIPThroughDHCP(config.MgmtInterfaceName)
			if err != nil {
				return fmt.Sprintf("Requesting IP through DHCP failed: %s", err.Error()), nil
			}
			logrus.Infof("DHCP test passed. Got IP: %s", addr)
			userInputData.Address = ""
			mgmtNetwork.IP = ""
			mgmtNetwork.SubnetMask = ""
			mgmtNetwork.Gateway = ""
			mgmtNetwork.MTU = 0
		}
		return "", nil
	}

	getNextPagePanel := func() []string {
		return []string{dnsServersPanel}
	}

	gotoNextPage := func(fromPanel string) error {
		if err := networkValidatorV.Show(); err != nil {
			return err
		}
		spinner := NewFocusSpinner(c.Gui, networkValidatorPanel, fmt.Sprintf("Applying network configuration..."))
		spinner.Start()
		go func(g *gocui.Gui) {
			msg, err := preGotoNextPage()
			if err != nil || msg != "" {
				var isErr bool
				var errMsg string
				if err != nil {
					isErr, errMsg = true, fmt.Sprintf("failed to execute preGotoNextPage hook: %s", err)
				} else {
					isErr, errMsg = true, msg
				}

				spinner.Stop(isErr, errMsg)
				// Go back to the panel that triggered gotoNextPage
				g.Update(func(_ *gocui.Gui) error {
					return showNext(c, fromPanel)
				})
			} else {
				spinner.Stop(false, "")
				g.Update(func(_ *gocui.Gui) error {
					closeThisPage()
					return showNext(c, getNextPagePanel()...)
				})
			}
		}(c.Gui)
		return nil
	}

	gotoPrevPage := func(_ *gocui.Gui, _ *gocui.View) error {
		closeThisPage()
		return showHostnamePage(c)
	}
	// askInterfaceV
	askInterfaceV.PreShow = func() error {
		askInterfaceV.Focus = true
		return c.setContentByName(titlePanel, networkTitle)
	}
	validateInterface := func() (string, error) {
		ifaces := askInterfaceV.GetMultiData()
		if len(ifaces) == 0 {
			return "Must select at least once interface", nil
		}
		interfaces := make([]config.NetworkInterface, 0, len(ifaces))
		for _, iface := range ifaces {
			switch nicState := getNICState(iface); nicState {
			case NICStateNotFound:
				return fmt.Sprintf("NIC %s not found", iface), nil
			case NICStateDown:
				return fmt.Sprintf("NIC %s is down", iface), nil
			case NICStateLowerDown:
				return fmt.Sprintf("NIC %s is down\nNetwork cable isn't plugged in", iface), nil
			}
			tmpInterface := config.NetworkInterface{
				Name: iface,
			}
			if err := tmpInterface.FindNetworkInterfaceHwAddr(); err != nil {
				return "", err
			}
			interfaces = append(interfaces, tmpInterface)
		}
		mgmtNetwork.Interfaces = interfaces
		return "", nil
	}
	interfaceVConfirm := gotoNextPanel(c, []string{askVlanIDPanel}, validateInterface)
	askInterfaceV.SetMulti(true)
	askInterfaceV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoPrevPage,
		gocui.KeyArrowDown: interfaceVConfirm,
		gocui.KeyEnter:     interfaceVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askInterfaceV.Panel, 3)
	c.AddElement(askInterfacePanel, askInterfaceV)

	// askVlanIDV
	askVlanIDV.PreShow = func() error {
		c.Gui.Cursor = true
		if mgmtNetwork.VlanID != 0 {
			askVlanIDV.Value = strconv.Itoa(mgmtNetwork.VlanID)
		} else {
			askVlanIDV.Value = ""
		}
		return nil
	}
	validateVlanID := func() (string, error) {
		vlanIDStr, err := askVlanIDV.GetData()
		if err != nil {
			return "", err
		}
		if vlanIDStr == "" {
			mgmtNetwork.VlanID = 0
			return "", nil
		}
		var vlanID int
		vlanID, err = strconv.Atoi(vlanIDStr)
		if err != nil {
			return ErrMsgVLANShouldBeANumberInRange, nil
		}
		// 0 is unset
		if vlanID < 0 || vlanID > 4094 {
			return ErrMsgVLANShouldBeANumberInRange, nil
		}
		mgmtNetwork.VlanID = vlanID
		return "", nil
	}
	askVlanIDVConfirm := gotoNextPanel(c, []string{askBondModePanel}, validateVlanID)
	askVlanIDV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, []string{askInterfacePanel}, validateVlanID),
		gocui.KeyArrowDown: askVlanIDVConfirm,
		gocui.KeyEnter:     askVlanIDVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askVlanIDV.Panel, 3)
	c.AddElement(askVlanIDPanel, askVlanIDV)

	// askBondModeV
	askBondModeV.PreShow = func() error {
		if mgmtNetwork.BondOptions == nil {
			askBondModeV.Value = config.BondModeActiveBackup
			mgmtNetwork.BondOptions = map[string]string{
				"mode":   config.BondModeActiveBackup,
				"miimon": "100",
			}
		}
		return nil
	}
	askBondModeVConfirm := func(_ *gocui.Gui, _ *gocui.View) error {
		mode, err := askBondModeV.GetData()
		mgmtNetwork.BondOptions = map[string]string{
			"mode":   mode,
			"miimon": "100",
		}
		if err != nil {
			return err
		}
		if err := showBondNote(); err != nil {
			return err
		}
		if mgmtNetwork.Method != config.NetworkMethodStatic {
			return showNext(c, askNetworkMethodPanel)
		}
		return showNext(c, mtuPanel, gatewayPanel, addressPanel, askNetworkMethodPanel)
	}
	askBondModeV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, []string{askVlanIDPanel}),
		gocui.KeyArrowDown: askBondModeVConfirm,
		gocui.KeyEnter:     askBondModeVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askBondModeV.Panel, 3)
	c.AddElement(askBondModePanel, askBondModeV)

	// askNetworkMethodV
	askNetworkMethodVConfirm := func(_ *gocui.Gui, _ *gocui.View) error {
		selected, err := askNetworkMethodV.GetData()
		if err != nil {
			return err
		}
		mgmtNetwork.Method = selected
		if selected == config.NetworkMethodStatic {
			return showNext(c, mtuPanel, gatewayPanel, addressPanel)
		}

		c.CloseElements(mtuPanel, gatewayPanel, addressPanel)
		return gotoNextPage(askNetworkMethodPanel)
	}
	askNetworkMethodV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, []string{askBondModePanel}),
		gocui.KeyArrowDown: askNetworkMethodVConfirm,
		gocui.KeyEnter:     askNetworkMethodVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askNetworkMethodV.Panel, 3)
	c.AddElement(askNetworkMethodPanel, askNetworkMethodV)

	// AddressV
	addressV.PreShow = func() error {
		c.Gui.Cursor = true
		addressV.Value = userInputData.Address
		return nil
	}
	validateAddress := func() (string, error) {
		address, err := addressV.GetData()
		if err != nil {
			return "", err
		}
		if err = checkStaticRequiredString("address", address); err != nil {
			return err.Error(), nil
		}
		ip, ipNet, err := net.ParseCIDR(address)
		if err != nil {
			return err.Error(), nil
		}
		mask := ipNet.Mask
		userInputData.Address = address
		mgmtNetwork.IP = ip.String()
		mgmtNetwork.SubnetMask = fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
		return "", nil
	}
	addressVConfirm := gotoNextPanel(c, []string{gatewayPanel}, validateAddress)
	addressV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: gotoNextPanel(c, []string{askNetworkMethodPanel}, func() (string, error) {
			userInputData.Address, err = addressV.GetData()
			return "", err
		}),
		gocui.KeyArrowDown: addressVConfirm,
		gocui.KeyEnter:     addressVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(addressV.Panel, 3)
	c.AddElement(addressPanel, addressV)

	// gatewayV
	gatewayV.PreShow = func() error {
		c.Gui.Cursor = true
		gatewayV.Value = mgmtNetwork.Gateway
		return nil
	}
	validateGateway := func() (string, error) {
		gateway, err := gatewayV.GetData()
		if err != nil {
			return "", err
		}
		if err = checkStaticRequiredString("gateway", gateway); err != nil {
			return err.Error(), nil
		}
		if err = checkIP(gateway); err != nil {
			return err.Error(), nil
		}
		mgmtNetwork.Gateway = gateway
		return "", nil
	}
	gatewayVConfirm := gotoNextPanel(c, []string{mtuPanel}, validateGateway)
	gatewayV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: gotoNextPanel(c, []string{addressPanel}, func() (string, error) {
			mgmtNetwork.Gateway, err = gatewayV.GetData()
			return "", err
		}),
		gocui.KeyArrowDown: gatewayVConfirm,
		gocui.KeyEnter:     gatewayVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(gatewayV.Panel, 3)
	c.AddElement(gatewayPanel, gatewayV)

	// mtuV
	mtuV.PreShow = func() error {
		c.Gui.Cursor = true
		if mgmtNetwork.MTU == 0 {
			mtuV.Value = ""
		} else {
			mtuV.Value = strconv.Itoa(mgmtNetwork.MTU)
		}
		return nil
	}
	validateMTU := func() (string, error) {
		var mtu int
		mtuStr, err := mtuV.GetData()
		if err != nil {
			return "", err
		}

		if mtuStr == "" {
			mtu = 0
		} else {
			mtu, err = strconv.Atoi(mtuStr)
			if err != nil {
				return ErrMsgMTUShouldBeANumber, nil
			}
		}

		if err = checkMTU(mtu); err != nil {
			return err.Error(), nil
		}
		mgmtNetwork.MTU = mtu
		return "", nil
	}
	mtuVConfirm := func(_ *gocui.Gui, _ *gocui.View) error {
		msg, err := validateMTU()
		if err != nil {
			return err
		}
		if msg != "" {
			return updateValidatorMessage(msg)
		}

		return gotoNextPage(mtuPanel)
	}
	mtuV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, []string{gatewayPanel}, validateMTU),
		gocui.KeyArrowDown: mtuVConfirm,
		gocui.KeyEnter:     mtuVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(mtuV.Panel, 3)
	c.AddElement(mtuPanel, mtuV)

	// bondNoteV
	bondNoteV.Wrap = true
	setLocation(bondNoteV, 0)
	c.AddElement(bondNotePanel, bondNoteV)

	// networkValidatorV
	networkValidatorV.FgColor = gocui.ColorRed
	networkValidatorV.Wrap = true
	setLocation(networkValidatorV, 0)
	c.AddElement(networkValidatorPanel, networkValidatorV)

	return nil
}

func getBondModeOptions() ([]widgets.Option, error) {
	return []widgets.Option{
		{
			Value: config.BondModeBalanceRR,
			Text:  config.BondModeBalanceRR,
		},
		{
			Value: config.BondModeActiveBackup,
			Text:  config.BondModeActiveBackup,
		},
		{
			Value: config.BondModeBalnaceXOR,
			Text:  config.BondModeBalnaceXOR,
		},
		{
			Value: config.BondModeBroadcast,
			Text:  config.BondModeBroadcast,
		},
		{
			Value: config.BondModeIEEE802_3ad,
			Text:  config.BondModeIEEE802_3ad,
		},
		{
			Value: config.BondModeBalanceTLB,
			Text:  config.BondModeBalanceTLB,
		},
		{
			Value: config.BondModeBalanceALB,
			Text:  config.BondModeBalanceALB,
		},
	}, nil
}

func getNetworkInterfaceOptions() ([]widgets.Option, error) {
	var options = []widgets.Option{}
	nics, err := getNICs()
	if err != nil {
		return nil, err
	}

	for _, nic := range nics {
		name := nic.Attrs().Name
		option := widgets.Option{
			Value: name,
			Text:  fmt.Sprintf("%s(%s, %s)", name, nic.Attrs().HardwareAddr.String(), nic.Attrs().OperState.String()),
		}
		options = append(options, option)
	}
	return options, nil
}

func getNetworkMethodOptions() ([]widgets.Option, error) {
	return []widgets.Option{
		{
			Value: config.NetworkMethodDHCP,
			Text:  networkMethodDHCPText,
		},
		{
			Value: config.NetworkMethodStatic,
			Text:  networkMethodStaticText,
		},
	}, nil
}

func addProxyPanel(c *Console) error {
	proxyV, err := widgets.NewInput(c.Gui, proxyPanel, "Proxy address", false)
	if err != nil {
		return err
	}
	proxyV.PreShow = func() error {
		c.Gui.Cursor = true
		proxyV.Value = os.Getenv("HTTP_PROXY")
		if err := c.setContentByName(titlePanel, "Optional: configure proxy"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, proxyNote)
	}
	proxyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			proxy, err := proxyV.GetData()
			if err != nil {
				return err
			}
			if proxy != "" {
				os.Setenv("HTTP_PROXY", proxy)
				os.Setenv("HTTPS_PROXY ", proxy)
			} else {
				os.Unsetenv("HTTP_PROXY")
				os.Unsetenv("HTTPS_PROXY")
			}
			proxyV.Close()
			noteV, err := c.GetElement(notePanel)
			if err != nil {
				return err
			}
			noteV.Close()
			return showNext(c, sshKeyPanel)
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			proxyV.Close()
			c.CloseElement(notePanel)
			return showNext(c, ntpServersPanel)
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
		c.Gui.Cursor = true
		cloudInitV.Value = c.config.Install.ConfigURL
		return c.setContentByName(titlePanel, "Optional: remote Harvester config")
	}
	gotoNextPage := func() error {
		cloudInitV.Close()
		return showNext(c, confirmInstallPanel)
	}
	cloudInitV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			configURL, err := cloudInitV.GetData()
			if err != nil {
				return err
			}
			c.config.Install.ConfigURL = configURL
			if configURL != "" {
				asyncTaskV, err := c.GetElement(spinnerPanel)
				if err != nil {
					return err
				}
				asyncTaskV.Close()
				asyncTaskV.Show()

				spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", configURL))
				spinner.Start()

				go func(g *gocui.Gui) {
					if _, err = getRemoteConfig(configURL); err != nil {
						spinner.Stop(true, err.Error())
						g.Update(func(_ *gocui.Gui) error {
							return showNext(c, cloudInitPanel)
						})
						return
					}
					spinner.Stop(false, "")
					g.Update(func(_ *gocui.Gui) error {
						return gotoNextPage()
					})
				}(c.Gui)
				return nil
			}
			return gotoNextPage()
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			cloudInitV.Close()
			return showNext(c, sshKeyPanel)
		},
	}
	cloudInitV.PostClose = func() error {
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(cloudInitPanel, cloudInitV)
	return nil
}

func addConfirmInstallPanel(c *Console) error {
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
	confirmV, err := widgets.NewSelect(c.Gui, confirmInstallPanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	confirmV.PreShow = func() error {
		installBytes, err := config.PrintInstall(*c.config)
		if err != nil {
			return err
		}
		options := fmt.Sprintf("install mode: %v\n", c.config.Install.Mode)
		options += fmt.Sprintf("hostname: %v\n", c.config.OS.Hostname)
		if userInputData.NTPServers != "" {
			options += fmt.Sprintf("ntp servers: %v\n", userInputData.NTPServers)
		}
		if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
			options += fmt.Sprintf("proxy address: %v\n", proxy)
		}
		if userInputData.SSHKeyURL != "" {
			options += fmt.Sprintf("ssh key url: %v\n", userInputData.SSHKeyURL)
		}
		options += string(installBytes)
		logrus.Debug("cfm cfg: ", fmt.Sprintf("%+v", c.config.Install))
		if !c.config.Install.Silent {
			if alreadyInstalled {
				confirmV.SetContent(options +
					"\nHarvester is already installed. It will be configured with \nthe above configuration.\n Continue?\n")
			} else if installModeOnly {
				confirmV.SetContent(options +
					"\nHarvester will be copied to local disk.\n No configuration will be performed.\n Continue?\n")
			} else {
				confirmV.SetContent(options +
					"\nYour disk will be formatted and Harvester will be installed with \nthe above configuration. Continue?\n")
			}
		}
		c.Gui.Cursor = false
		return c.setContentByName(titlePanel, "Confirm installation options")
	}
	confirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			confirmed, err := confirmV.GetData()
			if err != nil {
				return err
			}
			if confirmed == "no" {
				confirmV.Close()
				c.setContentByName(titlePanel, "")
				c.setContentByName(footerPanel, "")
				go util.SleepAndReboot()
				return c.setContentByName(notePanel, "Installation halted. Rebooting system in 5 seconds")
			}
			confirmV.Close()
			return showNext(c, installPanel)
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			confirmV.Close()
			if installModeOnly {
				return showNext(c, passwordConfirmPanel, passwordPanel)
			}
			return showNext(c, cloudInitPanel)
		},
	}
	c.AddElement(confirmInstallPanel, confirmV)
	return nil
}

func addConfirmUpgradePanel(c *Console) error {
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
	confirmV, err := widgets.NewSelect(c.Gui, confirmUpgradePanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	confirmV.PreShow = func() error {
		return c.setContentByName(titlePanel, fmt.Sprintf("Confirm upgrading Harvester to %s?", version.Version))
	}
	confirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			confirmed, err := confirmV.GetData()
			if err != nil {
				return err
			}
			confirmV.Close()
			if confirmed == "no" {
				return showNext(c, askCreatePanel)
			}
			return showNext(c, upgradePanel)
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			confirmV.Close()
			return showNext(c, askCreatePanel)
		},
	}
	c.AddElement(confirmUpgradePanel, confirmV)
	return nil
}

func addInstallPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	installV := widgets.NewPanel(c.Gui, installPanel)
	installV.PreShow = func() error {
		go func() {
			// in alreadyInstalled mode and auto configuration, the network is not available
			if alreadyInstalled && c.config.Automatic == true && c.config.ManagementInterface.Method == "dhcp" {
				configureInstallModeDHCP(c)
			}
			logrus.Info("Local config: ", c.config)
			if c.config.Install.ConfigURL != "" {
				printToPanel(c.Gui, fmt.Sprintf("Fetching %s...", c.config.Install.ConfigURL), installPanel)
				remoteConfig, err := retryRemoteConfig(c.config.Install.ConfigURL, c.Gui)
				if err != nil {
					logrus.Error(err)
					printToPanel(c.Gui, err.Error(), installPanel)
					return
				}
				logrus.Info("Remote config: ", remoteConfig)
				if err := c.config.Merge(*remoteConfig); err != nil {
					printToPanel(c.Gui, fmt.Sprintf("fail to merge config: %s", err), installPanel)
					return
				}
				logrus.Info("Local config (merged): ", c.config)

				if needToGetVIPFromDHCP(c.config.VipMode, c.config.Vip, c.config.VipHwAddr) {
					printToPanel(c.Gui, "Configuring network...", installPanel)
					if _, err := applyNetworks(c.config.ManagementInterface, c.config.Hostname); err != nil {
						printToPanel(c.Gui, fmt.Sprintf("can't apply networks: %s", err), installPanel)
						return
					}
					mgmtName := getManagementInterfaceName(c.config.ManagementInterface)
					vip, err := getVipThroughDHCP(mgmtName)
					if err != nil {
						printToPanel(c.Gui, fmt.Sprintf("fail to get vip: %s", err), installPanel)
						return
					}
					c.config.Vip = vip.ipv4Addr
					c.config.VipHwAddr = vip.hwAddr
				}
			}
			c.config.VipMode = strings.ToLower(c.config.VipMode)

			if c.config.Hostname == "" {
				c.config.Hostname = generateHostName()
			}
			if c.config.TTY == "" {
				c.config.TTY = getFirstConsoleTTY()
			}
			if c.config.ServerURL != "" {
				formatted, err := getFormattedServerURL(c.config.ServerURL)
				if err != nil {
					printToPanel(c.Gui, fmt.Sprintf("server url invalid: %s", err), installPanel)
					return
				}
				c.config.ServerURL = formatted
			}

			// lookup MAC Address to populate device names where needed
			// lookup device name to populate MAC Address
			tmpInterfaces := []config.NetworkInterface{}
			for _, iface := range c.config.ManagementInterface.Interfaces {
				if err := iface.FindNetworkInterfaceNameAndHwAddr(); err != nil {
					logrus.Error(err)
					printToPanel(c.Gui, err.Error(), installPanel)
					return
				}
				tmpInterfaces = append(tmpInterfaces, iface)
			}
			c.config.ManagementInterface.Interfaces = tmpInterfaces

			// We need ForceGPT because cOS only supports ForceGPT (--force-gpt) flag, not ForceMBR!
			c.config.ForceGPT = !c.config.ForceMBR

			// Clear the DataDisk field if it's identical to the installation disk
			if c.config.DataDisk == c.config.Device {
				c.config.DataDisk = ""
			}

			// case insensitive for network method and vip mode
			c.config.ManagementInterface.Method = strings.ToLower(c.config.ManagementInterface.Method)

			if err := validateConfig(ConfigValidator{}, c.config); err != nil {
				printToPanel(c.Gui, err.Error(), installPanel)
				return
			}

			webhooks, err := PrepareWebhooks(c.config.Webhooks, getWebhookContext(c.config))
			if err != nil {
				printToPanel(c.Gui, fmt.Sprintf("invalid webhook: %s", err), installPanel)
			}

			if alreadyInstalled {
				configureInstalledNode(c.Gui, c.config, webhooks)
			} else {
				doInstall(c.Gui, c.config, webhooks)
			}
		}()
		return c.setContentByName(footerPanel, "")
	}
	installV.Title = " Installing Harvester "
	installV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8*7)
	installV.Wrap = true
	installV.Autoscroll = true
	c.AddElement(installPanel, installV)
	installV.Frame = true
	return nil
}

func addSpinnerPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	asyncTaskV := widgets.NewPanel(c.Gui, spinnerPanel)
	asyncTaskV.SetLocation(maxX/8, maxY/8+7, maxX/8*7, maxY/8*7)
	asyncTaskV.Wrap = true
	c.AddElement(spinnerPanel, asyncTaskV)
	return nil
}

func addUpgradePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	upgradeV := widgets.NewPanel(c.Gui, upgradePanel)
	upgradeV.PreShow = func() error {
		go doUpgrade(c.Gui)
		return c.setContentByName(footerPanel, "")
	}
	upgradeV.Title = " Upgrading Harvester "
	upgradeV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8*7)
	c.AddElement(upgradePanel, upgradeV)
	upgradeV.Frame = true
	return nil
}

func addVIPPanel(c *Console) error {
	setLocation := createVerticalLocator(c)

	askVipMethodV, err := widgets.NewDropDown(c.Gui, askVipMethodPanel, askVipMethodLabel, getNetworkMethodOptions)
	if err != nil {
		return err
	}
	vipV, err := widgets.NewInput(c.Gui, vipPanel, vipLabel, false)
	if err != nil {
		return err
	}

	vipTextV := widgets.NewPanel(c.Gui, vipTextPanel)

	closeThisPage := func() {
		c.CloseElements(
			askVipMethodPanel,
			vipPanel,
			vipTextPanel)
	}

	gotoPrevPage := func(_ *gocui.Gui, _ *gocui.View) error {
		closeThisPage()
		return showNext(c, dnsServersPanel)
	}
	gotoNextPage := func(_ *gocui.Gui, _ *gocui.View) error {
		closeThisPage()
		return showNext(c, tokenPanel)
	}
	gotoVipPanel := func(g *gocui.Gui, _ *gocui.View) error {
		selected, err := askVipMethodV.GetData()
		if err != nil {
			return err
		}
		if selected == config.NetworkMethodDHCP {
			spinner := NewSpinner(c.Gui, vipTextPanel, "Requesting IP through DHCP...")
			spinner.Start()
			go func(g *gocui.Gui) {
				mgmtName := getManagementInterfaceName(c.config.ManagementInterface)
				vip, err := getVipThroughDHCP(mgmtName)
				if err != nil {
					spinner.Stop(true, err.Error())
					g.Update(func(_ *gocui.Gui) error {
						return showNext(c, askVipMethodPanel)
					})
					return
				}
				spinner.Stop(false, "")
				c.config.Vip = vip.ipv4Addr
				c.config.VipMode = selected
				c.config.VipHwAddr = vip.hwAddr
				g.Update(func(_ *gocui.Gui) error {
					return vipV.SetData(vip.ipv4Addr)
				})
			}(c.Gui)
		} else {
			vipTextV.SetContent("")
			g.Update(func(_ *gocui.Gui) error {
				return vipV.SetData("")
			})
			c.config.VipMode = config.NetworkMethodStatic
		}

		return showNext(c, vipPanel)
	}
	gotoVerifyIP := func(g *gocui.Gui, v *gocui.View) error {
		vip, err := vipV.GetData()
		if err != nil {
			return err
		}

		if c.config.VipMode == config.NetworkMethodDHCP {
			if vip != c.config.Vip {
				vipTextV.SetContent("Forbid to modify the VIP obtained through DHCP")
				return nil
			}
			return gotoNextPage(g, v)
		}

		// verify static IP
		if net.ParseIP(vip) == nil {
			vipTextV.SetContent(fmt.Sprintf("Invalid VIP: %s", vip))
			return nil
		}

		if vip != "" && vip == mgmtNetwork.IP {
			vipTextV.SetContent("VIP must not be the same as management NIC's IP")
			return nil
		}

		c.config.Vip = vip
		c.config.VipHwAddr = ""

		return gotoNextPage(g, v)
	}
	gotoAskVipMethodPanel := func(_ *gocui.Gui, _ *gocui.View) error {
		return showNext(c, askVipMethodPanel)
	}
	askVipMethodV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowDown: gotoVipPanel,
		gocui.KeyEnter:     gotoVipPanel,
		gocui.KeyEsc:       gotoPrevPage,
	}
	vipV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoAskVipMethodPanel,
		gocui.KeyArrowDown: gotoVerifyIP,
		gocui.KeyEnter:     gotoVerifyIP,
		gocui.KeyEsc:       gotoPrevPage,
	}

	askVipMethodV.PreShow = func() error {
		c.Gui.Cursor = true
		vipTextV.SetContent("")
		return c.setContentByName(titlePanel, vipTitle)
	}

	setLocation(askVipMethodV, 3)
	c.AddElement(askVipMethodPanel, askVipMethodV)

	setLocation(vipV, 3)
	c.AddElement(vipPanel, vipV)

	vipTextV.FgColor = gocui.ColorRed
	vipTextV.Focus = false
	vipTextV.Wrap = true
	setLocation(vipTextV, 0)
	c.AddElement(vipTextPanel, vipTextV)

	return nil
}

func addNTPServersPanel(c *Console) error {
	ntpServersV, err := widgets.NewInput(c.Gui, ntpServersPanel, ntpServersLabel, false)
	if err != nil {
		return err
	}

	ntpServersV.PreShow = func() error {
		c.Gui.Cursor = true
		ntpServersV.Value = userInputData.NTPServers
		if err = c.setContentByName(titlePanel, "Configure NTP Servers"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, ntpServersNote)
	}

	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return ntpServersV.Close()
	}
	gotoPrevPage := func(_ *gocui.Gui, _ *gocui.View) error {
		userInputData.HasCheckedNTPServers = false
		closeThisPage()
		return showNext(c, passwordConfirmPanel, passwordPanel)
	}
	gotoNextPage := func() error {
		userInputData.HasCheckedNTPServers = false
		closeThisPage()
		return showNext(c, proxyPanel)
	}
	gotoSpinnerErrorPage := func(g *gocui.Gui, spinner *Spinner, msg string) {
		spinner.Stop(true, msg)
		g.Update(func(_ *gocui.Gui) error {
			return showNext(c, ntpServersPanel)
		})
	}

	ntpServersV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			// get ntp servers input
			ntpServers, err := ntpServersV.GetData()
			if err != nil {
				return err
			}

			// When input servers can't be reached and users don't want to change it, we continue the process.
			if userInputData.NTPServers == ntpServers && userInputData.HasCheckedNTPServers == true {
				return gotoNextPage()
			}
			// reset HasCheckedNTPServers if users change input
			userInputData.HasCheckedNTPServers = false

			// init asyncTaskV
			asyncTaskV, err := c.GetElement(spinnerPanel)
			if err != nil {
				return err
			}
			asyncTaskV.Close()

			// focus on task panel to prevent input
			asyncTaskV.Show()

			spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking NTP Server: %q...", ntpServers))
			spinner.Start()

			go func(g *gocui.Gui) {
				if strings.TrimSpace(ntpServers) != ntpServers {
					gotoSpinnerErrorPage(g, spinner, fmt.Sprintf("There is space in input."))
					return
				}

				userInputData.HasCheckedNTPServers = true
				userInputData.NTPServers = ntpServers
				ntpServerList := strings.Split(ntpServers, ",")
				c.config.OS.NTPServers = ntpServerList

				if ntpServers == "" {
					gotoSpinnerErrorPage(g, spinner, fmt.Sprintf("Empty NTP Server is not recommended. Press Enter again to use current configuration anyway."))
					return
				}

				if err = validateNTPServers(ntpServerList); err != nil {
					logrus.Errorf("validate ntp servers: %v", err)
					gotoSpinnerErrorPage(g, spinner, fmt.Sprintf("Failed to reach NTP servers: %v. Press Enter again to use current configuration anyway, or change the value to revalidate.", err))
					return
				}
				if err = enableNTPServers(ntpServerList); err != nil {
					logrus.Errorf("enable ntp servers: %v", err)
					gotoSpinnerErrorPage(g, spinner, fmt.Sprintf("Failed to enalbe NTP servers: %v. Press Enter to proceed.", err))
					return
				}

				spinner.Stop(false, "")
				g.Update(func(_ *gocui.Gui) error {
					return gotoNextPage()
				})
			}(c.Gui)
			return nil
		},
		gocui.KeyEsc: gotoPrevPage,
	}
	ntpServersV.PostClose = func() error {
		if err := c.setContentByName(notePanel, ""); err != nil {
			return err
		}
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(ntpServersPanel, ntpServersV)

	return nil
}

func addDNSServersPanel(c *Console) error {
	dnsServersV, err := widgets.NewInput(c.Gui, dnsServersPanel, dnsServersLabel, false)
	if err != nil {
		return err
	}

	dnsServersV.PreShow = func() error {
		c.Gui.Cursor = true
		dnsServersV.Value = userInputData.DNSServers
		if err = c.setContentByName(titlePanel, "Configure DNS Servers"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, dnsServersNote)
	}

	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return dnsServersV.Close()
	}
	gotoPrevPage := func(_ *gocui.Gui, _ *gocui.View) error {
		closeThisPage()
		return showNetworkPage(c)
	}
	gotoNextPage := func() error {
		closeThisPage()
		if c.config.Install.Mode == config.ModeCreate {
			return showNext(c, vipTextPanel, vipPanel, askVipMethodPanel)
		}
		return showNext(c, serverURLPanel)
	}
	gotoSpinnerErrorPage := func(g *gocui.Gui, spinner *Spinner, msg string) {
		spinner.Stop(true, msg)
		g.Update(func(_ *gocui.Gui) error {
			return showNext(c, dnsServersPanel)
		})
	}

	dnsServersV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			// init asyncTaskV
			asyncTaskV, err := c.GetElement(spinnerPanel)
			if err != nil {
				return err
			}
			asyncTaskV.Close()

			// get dns servers
			dnsServers, err := dnsServersV.GetData()
			if err != nil {
				return err
			}
			dnsServers = strings.TrimSpace(dnsServers)
			userInputData.DNSServers = dnsServers

			// focus on task panel to prevent input
			asyncTaskV.Show()

			spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Setup DNS Servers: %q...", dnsServers))
			spinner.Start()

			go func(g *gocui.Gui) {
				if mgmtNetwork.Method == config.NetworkMethodStatic && dnsServers == "" {
					gotoSpinnerErrorPage(g, spinner, "DNS servers are required for static IP address")
					return
				}
				if dnsServers != "" {
					// check input syntax
					dnsServerList := strings.Split(dnsServers, ",")
					if err = checkIPList(dnsServerList); err != nil {
						gotoSpinnerErrorPage(g, spinner, err.Error())
						return
					}

					// setup dns
					if err = updateDNSServersAndReloadNetConfig(dnsServerList); err != nil {
						gotoSpinnerErrorPage(g, spinner, fmt.Sprintf("Failed to update DNS servers: %v.", err))
						return
					}

					c.config.OS.DNSNameservers = dnsServerList
				}
				spinner.Stop(false, "")
				g.Update(func(_ *gocui.Gui) error {
					return gotoNextPage()
				})
			}(c.Gui)
			return nil
		},
		gocui.KeyEsc: gotoPrevPage,
	}

	dnsServersV.PostClose = func() error {
		if err := c.setContentByName(notePanel, ""); err != nil {
			return err
		}
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(dnsServersPanel, dnsServersV)

	return nil
}

func configureInstallModeDHCP(c *Console) {
	netDef := c.config.Install.ManagementInterface
	// copy settings before application //
	mgmtNetwork.Interfaces = netDef.Interfaces
	if netDef.BondOptions == nil {
		mgmtNetwork.BondOptions = map[string]string{
			"mode":   config.BondModeBalanceTLB,
			"miimon": "100",
		}
	} else {
		mgmtNetwork.BondOptions = netDef.BondOptions
	}
	mgmtNetwork.Method = netDef.Method

	_, err := applyNetworks(
		mgmtNetwork,
		c.config.Hostname,
	)
	if err != nil {
		logrus.Error(err)
		printToPanel(c.Gui, fmt.Sprintf("error applying network configuration: %s", err.Error()), installPanel)
	}

	_, err = getIPThroughDHCP(config.MgmtInterfaceName)
	if err != nil {
		printToPanel(c.Gui, fmt.Sprintf("error getting DHCP address: %s", err.Error()), installPanel)
	}

	// if need vip via dhcp
	if c.config.Install.VipMode == config.NetworkMethodDHCP {
		vip, err := getVipThroughDHCP(config.MgmtInterfaceName)
		if err != nil {
			printToPanel(c.Gui, fmt.Sprintf("fail to get vip: %s", err), installPanel)
			return
		}
		c.config.Vip = vip.ipv4Addr
		c.config.VipHwAddr = vip.hwAddr
	}

}

func mergeCloudInit(c *config.HarvesterConfig) error {
	cloudConfig, err := config.ReadUserDataConfig()
	if err != nil {
		return err
	}
	if cloudConfig.Install.Automatic {
		c.Merge(cloudConfig)
		if cloudConfig.OS.Hostname != "" {
			c.OS.Hostname = cloudConfig.OS.Hostname
		}
		if cloudConfig.OS.Password != "" {
			c.OS.Password = cloudConfig.OS.Password
		}
	}

	return nil
}
