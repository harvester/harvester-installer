package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	yipSchema "github.com/mudler/yip/pkg/schema"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	cosLoginUser         = "rancher"
	manifestsDirectory   = "/var/lib/rancher/rke2/server/manifests/"
	canalConfig          = "rke2-canal-config.yaml"
	harvesterConfig      = "harvester-config.yaml"
	ntpdService          = "systemd-timesyncd"
	rancherdBootstrapDir = "/etc/rancher/rancherd/config.yaml.d/"

	networkConfigDirectory = "/etc/sysconfig/network/"
	ifcfgGlobPattern       = networkConfigDirectory + "ifcfg-*"
	ifrouteGlobPattern     = networkConfigDirectory + "ifroute-*"
)

var (
	// Following variables are replaced by ldflags
	RKE2Version            = ""
	HarvesterChartVersion  = ""
	MonitoringChartVersion = ""

	originalNetworkConfigs        = make(map[string][]byte)
	saveOriginalNetworkConfigOnce sync.Once
)

// ConvertToCOS converts HarvesterConfig to cOS configuration.
func ConvertToCOS(config *HarvesterConfig) (*yipSchema.YipConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
		return nil, err
	}

	// Overwrite rootfs layout
	rootfs := yipSchema.Stage{}
	if err := overwriteRootfsStage(config, &rootfs); err != nil {
		return nil, err
	}

	initramfs := yipSchema.Stage{
		SSHKeys:   make(map[string][]string),
		Users:     make(map[string]yipSchema.User),
		TimeSyncd: make(map[string]string),
	}

	// TOP
	if err := initRancherdStage(config, &initramfs); err != nil {
		return nil, err
	}

	// OS
	initramfs.SSHKeys[cosLoginUser] = cfg.OS.SSHAuthorizedKeys

	for _, ff := range cfg.OS.WriteFiles {
		perm, err := strconv.ParseUint(ff.RawFilePermissions, 8, 0)
		if err != nil {
			logrus.Warnf("fail to parse permission %s, use default permission.", err)
			perm = 0600
		}
		initramfs.Files = append(initramfs.Files, yipSchema.File{
			Path:        ff.Path,
			Content:     ff.Content,
			Permissions: uint32(perm),
			OwnerString: ff.Owner,
		})
	}

	initramfs.Hostname = cfg.OS.Hostname
	initramfs.Modules = cfg.OS.Modules
	initramfs.Sysctl = cfg.OS.Sysctls
	if len(cfg.OS.NTPServers) > 0 {
		initramfs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
		initramfs.Systemctl.Enable = append(initramfs.Systemctl.Enable, ntpdService)
	}
	if len(cfg.OS.DNSNameservers) > 0 {
		initramfs.Commands = append(initramfs.Commands, getAddStaticDNSServersCmd(cfg.OS.DNSNameservers))
	}

	if err := UpdateWifiConfig(&initramfs, cfg.OS.Wifi, false); err != nil {
		return nil, err
	}

	initramfs.Users[cosLoginUser] = yipSchema.User{
		PasswordHash: cfg.OS.Password,
	}

	initramfs.Environment = cfg.OS.Environment

	if err := UpdateNetworkConfig(&initramfs, cfg.Networks, false); err != nil {
		return nil, err
	}

	// mgmt interface: https://docs.rke2.io/install/network_options/#canal-options
	if cfg.Install.Mode == "create" {
		initramfs.Directories = append(initramfs.Directories, yipSchema.Directory{
			Path:        manifestsDirectory,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

		canalHelmChartConfig, err := render(canalConfig, config)
		if err != nil {
			return nil, err
		}
		initramfs.Files = append(initramfs.Files, yipSchema.File{
			Path:        manifestsDirectory + canalConfig,
			Content:     canalHelmChartConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
	}

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"rootfs":    {rootfs},
			"initramfs": {initramfs},
		},
	}

	return cosConfig, nil
}

func overwriteRootfsStage(config *HarvesterConfig, stage *yipSchema.Stage) error {
	content, err := render("cos-rootfs.yaml", config)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal([]byte(content), stage); err != nil {
		return err
	}

	return nil
}

func initRancherdStage(config *HarvesterConfig, stage *yipSchema.Stage) error {
	if config.RuntimeVersion == "" {
		config.RuntimeVersion = RKE2Version
	}
	if config.HarvesterChartVersion == "" {
		config.HarvesterChartVersion = HarvesterChartVersion
	}
	if config.MonitoringChartVersion == "" {
		config.MonitoringChartVersion = MonitoringChartVersion
	}

	stage.Directories = append(stage.Directories,
		yipSchema.Directory{
			Path:        "/etc/rancher/rke2/config.yaml.d",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

	rancherdConfig, err := render("rancherd-config.yaml", config)
	if err != nil {
		return err
	}
	stage.Files = append(stage.Files,
		yipSchema.File{
			Path:        RancherdConfigFile,
			Content:     rancherdConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	if config.Install.Mode == "create" {
		stage.Directories = append(stage.Directories,
			yipSchema.Directory{
				Path:        rancherdBootstrapDir,
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			},
		)

		bootstrapResources, err := genBootstrapResources(config)
		if err != nil {
			return err
		}
		for fileName, fileContent := range bootstrapResources {
			stage.Files = append(stage.Files,
				yipSchema.File{
					Path:        filepath.Join(rancherdBootstrapDir, fileName),
					Content:     fileContent,
					Permissions: 0600,
					Owner:       0,
					Group:       0,
				},
			)
		}
	}

	// RKE2 settings that can't be configured in rancherd
	rke2ServerConfig, err := render("rke2-90-harvester-server.yaml", config)
	if err != nil {
		return err
	}

	if config.ServerURL == "" {
		stage.Files = append(stage.Files,
			yipSchema.File{
				Path:        "/etc/rancher/rke2/config.yaml.d/90-harvester-server.yaml",
				Content:     rke2ServerConfig,
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			},
		)
	}

	rke2AgentConfig, err := render("rke2-90-harvester-agent.yaml", config)
	if err != nil {
		return err
	}
	stage.Files = append(stage.Files,
		yipSchema.File{
			Path:        "/etc/rancher/rke2/config.yaml.d/90-harvester-agent.yaml",
			Content:     rke2AgentConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	return nil
}

// RestoreOriginalNetworkConfig restores the previous state of network
// configurations saved by `SaveOriginalNetworkConfig`.
func RestoreOriginalNetworkConfig() error {
	if len(originalNetworkConfigs) == 0 {
		return nil
	}

	remove := func(pattern string) error {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	}

	if err := remove(ifrouteGlobPattern); err != nil {
		return err
	}
	if err := remove(ifcfgGlobPattern); err != nil {
		return err
	}

	for name, bytes := range originalNetworkConfigs {
		if err := ioutil.WriteFile(fmt.Sprintf("/etc/sysconfig/network/%s", name), bytes, os.FileMode(0600)); err != nil {
			return err
		}
	}
	return nil
}

// SaveOriginalNetworkConfig saves the current state of network configurations.
// Namely
// - all `/etc/sysconfig/network/ifroute-*` files, and
// - all `/etc/sysconfig/network/ifcfg-*` files
//
// It can only be invoked once for the whole lifetime of this program.
func SaveOriginalNetworkConfig() error {
	var err error

	saveOriginalNetworkConfigOnce.Do(func() {
		save := func(pattern string) error {
			filepaths, err := filepath.Glob(pattern)
			if err != nil {
				return err
			}
			for _, path := range filepaths {
				if bytes, err := ioutil.ReadFile(path); err != nil {
					return err
				} else {
					originalNetworkConfigs[filepath.Base(path)] = bytes
				}
			}
			return nil
		}

		if err = save(ifrouteGlobPattern); err != nil {
			return
		}
		err = save(ifcfgGlobPattern)
		return
	})

	return err
}

// UpdateNetworkConfig updates a cOS config stage to include steps that:
// - generates wicked interface files (`/etc/sysconfig/network/ifcfg-*` and `ifroute-*`)
// - manipulates nameservers in `/etc/resolv.conf`.
// - call `wicked ifreload all` if `run` flag is true.
func UpdateNetworkConfig(stage *yipSchema.Stage, networks map[string]Network, run bool) error {
	mgmtNetwork, ok := networks[MgmtInterfaceName]
	if !ok {
		return errors.New("no management network defined")
	}
	if len(mgmtNetwork.Interfaces) == 0 {
		return errors.New("no slave defined for management network bond")
	}

	for name, network := range networks {
		switch network.Method {
		case NetworkMethodDHCP, NetworkMethodStatic, NetworkMethodNone:
		default:
			return fmt.Errorf("unsupported network method %s", network.Method)
		}

		var err error
		if len(network.Interfaces) > 0 {
			err = updateBond(stage, name, &network)
		} else {
			err = updateNIC(stage, name, &network)
		}
		if network.VlanID != 0 {
			err = updateVLAN(stage, name, &network)
		}
		if err != nil {
			return err
		}

		switch network.Method {
		case NetworkMethodStatic:
			// default gateway for static mode
			stage.Files = append(stage.Files, yipSchema.File{
				Path:        fmt.Sprintf("/etc/sysconfig/network/ifroute-%s", name),
				Content:     fmt.Sprintf("default %s - %s\n", network.Gateway, name),
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			})
		case NetworkMethodDHCP, NetworkMethodNone:
			stage.Commands = append(stage.Commands, fmt.Sprintf("rm -f /etc/sysconfig/network/ifroute-%s", name))
		}
	}

	if run {
		stage.Commands = append(stage.Commands, "wicked ifreload all")

		// in case wicked config is not changed and netconfig is not called
		stage.Commands = append(stage.Commands, "netconfig update")
	}

	return nil
}

func updateNIC(stage *yipSchema.Stage, name string, network *Network) error {
	ifcfg, err := render("wicked-ifcfg-eth", network)
	if err != nil {
		return err
	}

	stage.Files = append(stage.Files, yipSchema.File{
		Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s", name),
		Content:     ifcfg,
		Permissions: 0600,
		Owner:       0,
		Group:       0,
	})
	return nil
}

func updateBond(stage *yipSchema.Stage, name string, network *Network) error {
	// Adding default NIC bonding options if no options are provided (usually happened under PXE
	// installation). Missing them would make bonding interfaces unusable.
	if network.BondOptions == nil {
		logrus.Infof("Adding default NIC bonding options for \"%s\"", name)
		network.BondOptions = map[string]string{
			"mode":   BondModeBalanceTLB,
			"miimon": "100",
		}
	}

	// Set default route for management bond
	if name == MgmtInterfaceName {
		network.DefaultRoute = true
	}

	ifcfg, err := render("wicked-ifcfg-bond-master", network)
	if err != nil {
		return err
	}

	// bond master
	stage.Files = append(stage.Files, yipSchema.File{
		Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s", name),
		Content:     ifcfg,
		Permissions: 0600,
		Owner:       0,
		Group:       0,
	})

	// bond slaves
	for _, iface := range network.Interfaces {
		ifcfg, err := render("wicked-ifcfg-bond-slave", iface)
		if err != nil {
			return err
		}
		stage.Files = append(stage.Files, yipSchema.File{
			Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s", iface.Name),
			Content:     ifcfg,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
	}

	return nil
}

func updateVLAN(stage *yipSchema.Stage, name string, network *Network) error {
	n := struct {
		Name string
		Network
	}{
		Name:    name,
		Network: *network,
	}
	ifcfg, err := render("wicked-ifcfg-vlan", n)
	if err != nil {
		return err
	}

	stage.Files = append(stage.Files, yipSchema.File{
		Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-vlan%d", network.VlanID),
		Content:     ifcfg,
		Permissions: 0600,
		Owner:       0,
		Group:       0,
	})
	return nil
}

func UpdateWifiConfig(stage *yipSchema.Stage, wifis []Wifi, run bool) error {
	if len(wifis) == 0 {
		return nil
	}

	var interfaces []string
	for i, wifi := range wifis {
		iface := fmt.Sprintf("wlan%d", i)

		ifcfg, err := render("wicked-ifcfg-wlan", wifi)
		if err != nil {
			return err
		}
		stage.Files = append(stage.Files, yipSchema.File{
			Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s", iface),
			Content:     ifcfg,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

		interfaces = append(interfaces, iface)
	}

	if run {
		stage.Commands = append(stage.Commands, fmt.Sprintf("wicked ifreload %s", strings.Join(interfaces, " ")))
	}

	return nil
}

func getAddStaticDNSServersCmd(servers []string) string {
	return fmt.Sprintf(`sed -i 's/^NETCONFIG_DNS_STATIC_SERVERS.*/NETCONFIG_DNS_STATIC_SERVERS="%s"/' /etc/sysconfig/network/config`, strings.Join(servers, " "))
}

func (c *HarvesterConfig) ToCosInstallEnv() ([]string, error) {
	return ToEnv("_COS_INSTALL_", c.Install)
}

// Returns Rancherd bootstrap resources
// map: fileName -> fileContent
func genBootstrapResources(config *HarvesterConfig) (map[string]string, error) {
	bootstrapConfs := make(map[string]string, 4)

	for _, templateName := range []string{
		"10-harvester.yaml",
		"11-monitoring-crd.yaml",
		"13-monitoring.yaml",
		"20-harvester-settings.yaml",
	} {
		rendered, err := render("rancherd-"+templateName, config)
		if err != nil {
			return nil, err
		}

		bootstrapConfs[templateName] = rendered
	}

	// These are not templates but I still put them here for consistency
	for _, templateName := range []string{
		"12-monitoring-dashboard.yaml",
		"14-monitoring-harvester-exporter.yaml",
	} {
		templBytes, err := templFS.ReadFile(filepath.Join(templateFolder, "rancherd-"+templateName))
		if err != nil {
			return nil, err
		}
		bootstrapConfs[templateName] = string(templBytes)
	}

	return bootstrapConfs, nil
}
