package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/harvester/harvester-installer/pkg/util"
	yipSchema "github.com/mudler/yip/pkg/schema"
	"github.com/sirupsen/logrus"
)

const (
	cosLoginUser = "rancher"
)

var (
	// RKE2Version is replaced by ldflags
	RKE2Version = ""
)

// ConvertToCOS converts HarvesterConfig to cOS configuration.
func ConvertToCOS(config *HarvesterConfig) (*yipSchema.YipConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
		return nil, err
	}

	preStage := yipSchema.Stage{}
	preStage.Commands = append(preStage.Commands, "rm -f /etc/sysconfig/network/ifcfg-eth0")

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
	initramfs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
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

	// ensure network that contains mgmtInterface exists
	if cfg.MgmtInterface != "" {
		mgmtInterfaceNetwork := false
		for _, network := range cfg.Networks {
			if network.Interface == cfg.MgmtInterface {
				mgmtInterfaceNetwork = true
				break
			}
		}

		if !mgmtInterfaceNetwork {
			cfg.Networks = append(cfg.Networks, Network{
				Interface: cfg.MgmtInterface,
				Method:    NetworkMethodDHCP,
			})
		}
	}

	if err := UpdateNetworkConfig(&initramfs, cfg.Networks, false); err != nil {
		return nil, err
	}

	// mgmt interface: https://docs.rke2.io/install/network_options/#canal-options
	if cfg.Install.Mode == "create" && cfg.Install.MgmtInterface != "" {
		canalHelmChartConfig, err := render("rke2-canal-config.yaml", config)
		if err != nil {
			return nil, err
		}
		initramfs.Directories = append(initramfs.Directories, yipSchema.Directory{
			Path:        "/var/lib/rancher/rke2/server/manifests/",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
		initramfs.Files = append(initramfs.Files, yipSchema.File{
			Path:        "/var/lib/rancher/rke2/server/manifests/rke2-canal-config.yaml",
			Content:     canalHelmChartConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
	}

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"initramfs": {preStage, initramfs},
		},
	}

	return cosConfig, nil
}

func initRancherdStage(config *HarvesterConfig, stage *yipSchema.Stage) error {
	if config.RuntimeVersion == "" {
		config.RuntimeVersion = RKE2Version
	}

	rancherdConfig, err := render("rancherd-config.yaml", config)
	if err != nil {
		return err
	}

	stage.Directories = append(stage.Directories,
		yipSchema.Directory{
			Path:        "/etc/rancher/rancherd",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		}, yipSchema.Directory{
			Path:        "/etc/rancher/rke2/config.yaml.d",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

	stage.Files = append(stage.Files,
		yipSchema.File{
			Path:        RancherdConfigFile,
			Content:     rancherdConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

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

// UpdateNetworkConfig updates a cOS config stage to include steps that:
// - generates wicked interface files (`/etc/sysconfig/network/ifcfg-*` and `ifroute-*`)
// - manipulates nameservers in `/etc/resolv.conf`.
// - call `wicked ifreload <interface...>` if `run` flag is true.
func UpdateNetworkConfig(stage *yipSchema.Stage, networks []Network, run bool) error {
	var interfaces []string
	var staticDNSServers []string

	for _, network := range networks {
		interfaces = append(interfaces, network.Interface)
		var templ string
		switch network.Method {
		case NetworkMethodDHCP:
			templ = "wicked-ifcfg-dhcp"
		case NetworkMethodStatic:
			templ = "wicked-ifcfg-static"
		default:
			return fmt.Errorf("unsupported network method %s", network.Method)
		}

		ifcfg, err := render(templ, network)
		if err != nil {
			return err
		}

		stage.Files = append(stage.Files, yipSchema.File{
			Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s", network.Interface),
			Content:     ifcfg,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

		// default gateway for static mode
		if network.Method == NetworkMethodStatic {
			stage.Files = append(stage.Files, yipSchema.File{
				Path:        fmt.Sprintf("/etc/sysconfig/network/ifroute-%s", network.Interface),
				Content:     fmt.Sprintf("default %s - %s\n", network.Gateway, network.Interface),
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			})
		}

		if network.Method == NetworkMethodDHCP {
			stage.Commands = append(stage.Commands, fmt.Sprintf("rm -f /etc/sysconfig/network/ifroute-%s", network.Interface))
		}

		for _, nameServer := range network.DNSNameservers {
			if util.StringSliceContains(staticDNSServers, nameServer) {
				continue
			}
			staticDNSServers = append(staticDNSServers, nameServer)
		}
	}

	// Set static DNS servers before wicked reload
	if len(staticDNSServers) > 0 {
		// Not using stage.Environment because it's run after stage.Commands in Yip
		stage.Commands = append(stage.Commands, getAddStaticDNSServersCmd(staticDNSServers))
	}

	if run {
		stage.Commands = append(stage.Commands, fmt.Sprintf("wicked ifreload %s", strings.Join(interfaces, " ")))

		// in case wicked config is not changed and netconfig is not called
		stage.Commands = append(stage.Commands, "netconfig update")
	}

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
	return ToEnv("COS_INSTALL_", c.Install)
}
