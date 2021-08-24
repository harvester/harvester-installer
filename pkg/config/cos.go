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

// ConvertToCOS converts HarvesterConfig to cOS configuration.
func ConvertToCOS(config *HarvesterConfig) (*yipSchema.YipConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
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
	initramfs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
	initramfs.Dns.Nameservers = cfg.OS.DNSNameservers

	// TODO(kiefer): wicked WIFI?
	// cloudConfig.K3OS.Wifi = copyWifi(cfg.OS.Wifi)

	initramfs.Users[cosLoginUser] = yipSchema.User{
		PasswordHash: cfg.OS.Password,
	}

	initramfs.Environment = cfg.OS.Environment

	// TODO(kiefer): Install
	if len(cfg.Networks) > 0 {
		if err := UpdateNetworkConfig(&initramfs, cfg.Networks, false); err != nil {
			return nil, err
		}
	}

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"initramfs": {initramfs},
		},
	}

	return cosConfig, nil
}

func initRancherdStage(config *HarvesterConfig, stage *yipSchema.Stage) error {
	rancherdConfig, err := render("rancherd-config.yaml", config)
	if err != nil {
		return err
	}

	rke2Config, err := render("rke2-99-harvester.yaml", config)
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
			Path:        "/etc/rancher/rancherd/config.yaml",
			Content:     rancherdConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	// server role: add network settings
	if config.ServerURL == "" {
		stage.Files = append(stage.Files,
			yipSchema.File{
				Path:        "/etc/rancher/rke2/config.yaml.d/99-harvester.yaml",
				Content:     rke2Config,
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			},
		)
	}

	return nil
}

// UpdateNetworkConfig updates a cOS config stage to include steps that:
// - generates wicked interface files (`/etc/sysconfig/network/ifcfg-*` and `ifroute-*`)
// - manipulates nameservers in `/etc/resolv.conf`.
// - call `wicked ifreload <interface...>` if `run` flag is true.
func UpdateNetworkConfig(stage *yipSchema.Stage, networks []Network, run bool) error {
	var interfaces []string

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

		if len(network.DNSNameservers) > 0 {
			for _, nameServer := range network.DNSNameservers {
				if util.StringSliceContains(stage.Dns.Nameservers, nameServer) {
					continue
				}
				stage.Dns.Nameservers = append(stage.Dns.Nameservers, nameServer)
			}
		}
	}

	if run {
		stage.Commands = append(stage.Commands, fmt.Sprintf("wicked ifreload %s", strings.Join(interfaces, " ")))
	}

	return nil
}
