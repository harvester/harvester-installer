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
	"gopkg.in/yaml.v3"

	"github.com/harvester/harvester-installer/pkg/util"
)

const (
	cosLoginUser         = "rancher"
	manifestsDirectory   = "/var/lib/rancher/rke2/server/manifests/"
	harvesterConfig      = "harvester-config.yaml"
	ntpdService          = "systemd-timesyncd"
	timeWaitSyncService  = "systemd-time-wait-sync"
	rancherdBootstrapDir = "/etc/rancher/rancherd/config.yaml.d/"

	networkConfigDirectory = "/etc/sysconfig/network/"
	ifcfgGlobPattern       = networkConfigDirectory + "ifcfg-*"
	ifrouteGlobPattern     = networkConfigDirectory + "ifroute-*"
)

var (
	// Following variables are replaced by ldflags
	RKE2Version            = ""
	RancherVersion         = ""
	HarvesterChartVersion  = ""
	MonitoringChartVersion = ""
	LoggingChartVersion    = ""

	originalNetworkConfigs        = make(map[string][]byte)
	saveOriginalNetworkConfigOnce sync.Once
)

// refer: https://github.com/rancher/elemental-cli/blob/v0.1.0/config.yaml.example
type ElementalConfig struct {
	Install ElementalInstallSpec `yaml:"install,omitempty"`
}

type ElementalInstallSpec struct {
	Target          string                     `yaml:"target,omitempty"`
	Firmware        string                     `yaml:"firmware,omitempty"`
	PartTable       string                     `yaml:"part-table,omitempty"`
	Partitions      *ElementalDefaultPartition `yaml:"partitions,omitempty"`
	ExtraPartitions []ElementalPartition       `yaml:"extra-partitions,omitempty"`
	CloudInit       string                     `yaml:"cloud-init,omitempty"`
	Tty             string                     `yaml:"tty,omitempty"`
}

type ElementalDefaultPartition struct {
	OEM        *ElementalPartition `yaml:"oem,omitempty"`
	State      *ElementalPartition `yaml:"state,omitempty"`
	Recovery   *ElementalPartition `yaml:"recovery,omitempty"`
	Persistent *ElementalPartition `yaml:"persistent,omitempty"`
}

type ElementalPartition struct {
	FilesystemLabel string `yaml:"label,omitempty"`
	Size            uint   `yaml:"size,omitempty"`
	FS              string `yaml:"fs,omitempty"`
}

func NewElementalConfig() *ElementalConfig {
	return &ElementalConfig{}
}

func ConvertToElementalConfig(config *HarvesterConfig) (*ElementalConfig, error) {
	elementalConfig := NewElementalConfig()

	if config.Install.ForceEFI {
		elementalConfig.Install.Firmware = "efi"
	}

	elementalConfig.Install.PartTable = "gpt"
	if !config.Install.ForceGPT {
		elementalConfig.Install.PartTable = "mbr"
	}

	resolvedDevPath, err := filepath.EvalSymlinks(config.Install.Device)
	if err != nil {
		return nil, err
	}
	elementalConfig.Install.Target = resolvedDevPath
	elementalConfig.Install.CloudInit = config.Install.ConfigURL
	elementalConfig.Install.Tty = config.Install.TTY

	return elementalConfig, nil
}

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
		Users:     make(map[string]yipSchema.User),
		TimeSyncd: make(map[string]string),
	}

	// TOP
	if err := initRancherdStage(config, &initramfs); err != nil {
		return nil, err
	}

	// OS
	for _, ff := range cfg.OS.WriteFiles {
		perm, err := strconv.ParseUint(ff.RawFilePermissions, 8, 32)
		if err != nil {
			logrus.Warnf("fail to parse permission %s, use default permission.", err)
			perm = 0600
		}

		initramfs.Files = append(initramfs.Files, yipSchema.File{
			Path:        ff.Path,
			Content:     ff.Content,
			Encoding:    ff.Encoding,
			Permissions: uint32(perm),
			OwnerString: ff.Owner,
		})
	}

	initramfs.Hostname = cfg.OS.Hostname
	initramfs.Sysctl = cfg.OS.Sysctls
	if len(cfg.OS.NTPServers) > 0 {
		initramfs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
		initramfs.Systemctl.Enable = append(initramfs.Systemctl.Enable, ntpdService)
		initramfs.Systemctl.Enable = append(initramfs.Systemctl.Enable, timeWaitSyncService)
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

	// Use modprobe to load modules as a temporary solution
	for _, module := range cfg.OS.Modules {
		initramfs.Commands = append(initramfs.Commands, "modprobe "+module)
	}

	_, err = UpdateManagementInterfaceConfig(&initramfs, cfg.ManagementInterface, false)
	if err != nil {
		return nil, err
	}

	// After network is available
	afterNetwork := yipSchema.Stage{
		SSHKeys: make(map[string][]string),
	}
	afterNetwork.SSHKeys[cosLoginUser] = cfg.OS.SSHAuthorizedKeys

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"rootfs":    {rootfs},
			"initramfs": {initramfs},
			"network":   {afterNetwork},
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
	if config.RancherVersion == "" {
		config.RancherVersion = RancherVersion
	}
	if config.HarvesterChartVersion == "" {
		config.HarvesterChartVersion = HarvesterChartVersion
	}
	if config.MonitoringChartVersion == "" {
		config.MonitoringChartVersion = MonitoringChartVersion
	}

	if config.LoggingChartVersion == "" {
		config.LoggingChartVersion = LoggingChartVersion
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

	// RKE2 settings of kube-audit
	rke2KubeAuditConfig, err := render("rke2-92-harvester-kube-audit-policy.yaml", config)
	if err != nil {
		return err
	}
	stage.Files = append(stage.Files,
		yipSchema.File{
			Path:        "/etc/rancher/rke2/config.yaml.d/92-harvester-kube-audit-policy.yaml",
			Content:     rke2KubeAuditConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	rke2AgentConfig, err := render("rke2-90-harvester-agent.yaml", config)
	if err != nil {
		return err
	}

	// remove space, so we don't get result like |2 or |4 in the yaml
	rke2AgentConfig = strings.TrimSpace(rke2AgentConfig)
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
				bytes, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				originalNetworkConfigs[filepath.Base(path)] = bytes
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

// UpdateManagementInterfaceConfig updates a cOS config stage to include steps that:
// - generates wicked interface files (`/etc/sysconfig/network/ifcfg-*` and `ifroute-*`)
// - manipulates nameservers in `/etc/resolv.conf`.
// - call `wicked ifreload all` if `run` flag is true.
func UpdateManagementInterfaceConfig(stage *yipSchema.Stage, mgmtInterface Network, run bool) (string, error) {
	if len(mgmtInterface.Interfaces) == 0 {
		return "", errors.New("no slave defined for management network bond")
	}

	switch mgmtInterface.Method {
	case NetworkMethodDHCP, NetworkMethodStatic, NetworkMethodNone:
	default:
		return "", fmt.Errorf("unsupported network method %s", mgmtInterface.Method)
	}

	if len(mgmtInterface.Interfaces) > 0 {
		bondMgmt := Network{
			Interfaces:  mgmtInterface.Interfaces,
			Method:      NetworkMethodNone,
			BondOptions: mgmtInterface.BondOptions,
			MTU:         mgmtInterface.MTU,
		}
		if err := updateBond(stage, MgmtBondInterfaceName, &bondMgmt); err != nil {
			return "", err
		}
	}

	if err := updateBridge(stage, MgmtInterfaceName, &mgmtInterface); err != nil {
		return "", err
	}

	name := MgmtInterfaceName
	if mgmtInterface.VlanID >= 2 && mgmtInterface.VlanID <= 4094 {
		name = fmt.Sprintf("%s.%d", name, mgmtInterface.VlanID)
	}

	switch mgmtInterface.Method {
	case NetworkMethodStatic:
		// default gateway for static mode
		stage.Files = append(stage.Files, yipSchema.File{
			Path:        fmt.Sprintf("/etc/sysconfig/network/ifroute-%s", name),
			Content:     fmt.Sprintf("default %s - %s\n", mgmtInterface.Gateway, name),
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
	case NetworkMethodDHCP, NetworkMethodNone:
		stage.Commands = append(stage.Commands, fmt.Sprintf("rm -f /etc/sysconfig/network/ifroute-%s", name))
	}

	if run {
		stage.Commands = append(stage.Commands, "wicked ifreload all")

		// in case wicked config is not changed and netconfig is not called
		stage.Commands = append(stage.Commands, "netconfig update")
	}

	return name, nil
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

	// setup post up script
	stage.Directories = append(stage.Directories, yipSchema.Directory{
		Path:        "/etc/wicked/scripts",
		Permissions: 0644,
		Owner:       0,
		Group:       0,
	})

	postUpScript, err := render("wicked-setup-bond.sh", MgmtInterfaceName)
	if err != nil {
		return err
	}
	stage.Files = append(stage.Files, yipSchema.File{
		Path:        "/etc/wicked/scripts/setup_bond.sh",
		Content:     postUpScript,
		Permissions: 0755,
		Owner:       0,
		Group:       0,
	})

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

func updateBridge(stage *yipSchema.Stage, name string, mgmtNetwork *Network) error {
	// add Bridge named MgmtInterfaceName and attach Bond named MgmtBondInterfaceName to bridge

	needVlanInterface := false
	// pvid is always 1, if vlan id is 1, it means untagged vlan.
	if mgmtNetwork.VlanID >= 2 && mgmtNetwork.VlanID <= 4094 {
		needVlanInterface = true
	}

	// setup pre up script
	stage.Directories = append(stage.Directories, yipSchema.Directory{
		Path:        "/etc/wicked/scripts",
		Permissions: 0644,
		Owner:       0,
		Group:       0,
	})

	preUpScript, err := render("wicked-setup-bridge.sh", MgmtBondInterfaceName)
	if err != nil {
		return err
	}
	stage.Files = append(stage.Files, yipSchema.File{
		Path:        "/etc/wicked/scripts/setup_bridge.sh",
		Content:     preUpScript,
		Permissions: 0755,
		Owner:       0,
		Group:       0,
	})

	bridgeMgmt := Network{
		Interfaces:   mgmtNetwork.Interfaces,
		Method:       mgmtNetwork.Method,
		IP:           mgmtNetwork.IP,
		SubnetMask:   mgmtNetwork.SubnetMask,
		Gateway:      mgmtNetwork.Gateway,
		DefaultRoute: !needVlanInterface,
		MTU:          mgmtNetwork.MTU,
	}

	if needVlanInterface {
		bridgeMgmt.Method = NetworkMethodNone
	}

	// add bridge
	bridgeData := map[string]interface{}{
		"Bridge": bridgeMgmt,
		"Bond":   MgmtBondInterfaceName,
	}
	var ifcfg string
	ifcfg, err = render("wicked-ifcfg-bridge", bridgeData)
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

	// add vlan interface
	if needVlanInterface {
		mgmtNetwork.DefaultRoute = true
		vlanData := map[string]interface{}{
			"BridgeName": name,
			"Vlan":       mgmtNetwork,
		}
		ifcfg, err = render("wicked-ifcfg-vlan", vlanData)
		if err != nil {
			return err
		}
		stage.Files = append(stage.Files, yipSchema.File{
			Path:        fmt.Sprintf("/etc/sysconfig/network/ifcfg-%s.%d", name, mgmtNetwork.VlanID),
			Content:     ifcfg,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})
	}

	return nil
}

func UpdateWifiConfig(stage *yipSchema.Stage, wifis []Wifi, run bool) error {
	if len(wifis) == 0 {
		return nil
	}

	interfaces := make([]string, len(wifis))
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
	return ToEnv("HARVESTER_", c.Install)
}

// Returns Rancherd bootstrap resources
// map: fileName -> fileContent
func genBootstrapResources(config *HarvesterConfig) (map[string]string, error) {
	bootstrapConfs := make(map[string]string, 4)

	for _, templateName := range []string{
		"10-harvester.yaml",
		"11-monitoring-crd.yaml",
		"13-monitoring.yaml",
		"14-logging-crd.yaml",
		"15-logging.yaml",
		"20-harvester-settings.yaml",
	} {
		rendered, err := render("rancherd-"+templateName, config)
		if err != nil {
			return nil, err
		}

		bootstrapConfs[templateName] = rendered
	}
	// It's not a template but I still put it here for consistency

	for _, templateName := range []string{
		"12-monitoring-dashboard.yaml",
		"22-addons.yaml",
	} {
		templBytes, err := templFS.ReadFile(filepath.Join(templateFolder, "rancherd-"+templateName))
		if err != nil {
			return nil, err
		}
		bootstrapConfs[templateName] = string(templBytes)
	}

	return bootstrapConfs, nil
}

func calcCosPersistentPartSize(diskSizeGiB uint64) (uint64, error) {
	switch {
	case diskSizeGiB < HardMinDiskSizeGiB:
		return 0, fmt.Errorf("disk too small: %dGB. Minimum %dGB is required", diskSizeGiB, HardMinDiskSizeGiB)
	case diskSizeGiB < SoftMinDiskSizeGiB:
		d := MinCosPartSizeGiB / float64(SoftMinDiskSizeGiB-HardMinDiskSizeGiB)
		partSizeGiB := MinCosPartSizeGiB + float64(diskSizeGiB-HardMinDiskSizeGiB)*d
		return uint64(partSizeGiB), nil
	default:
		partSizeGiB := NormalCosPartSizeGiB + ((diskSizeGiB-100)/100)*10
		if partSizeGiB > 100 {
			partSizeGiB = 100
		}
		return partSizeGiB, nil
	}
}

func CreateRootPartitioningLayout(elementalConfig *ElementalConfig, devPath string) (*ElementalConfig, error) {
	diskSizeBytes, err := util.GetDiskSizeBytes(devPath)
	if err != nil {
		return nil, err
	}

	cosPersistentSizeGiB, err := calcCosPersistentPartSize(diskSizeBytes >> 30)
	if err != nil {
		return nil, err
	}

	elementalConfig.Install.Partitions = &ElementalDefaultPartition{
		OEM: &ElementalPartition{
			FilesystemLabel: "COS_OEM",
			Size:            50,
			FS:              "ext4",
		},
		State: &ElementalPartition{
			FilesystemLabel: "COS_STATE",
			Size:            15360,
			FS:              "ext4",
		},
		Recovery: &ElementalPartition{
			FilesystemLabel: "COS_RECOVERY",
			Size:            8192,
			FS:              "ext4",
		},
		Persistent: &ElementalPartition{
			FilesystemLabel: "COS_PERSISTENT",
			Size:            uint(cosPersistentSizeGiB << 10),
			FS:              "ext4",
		},
	}

	elementalConfig.Install.ExtraPartitions = []ElementalPartition{
		{
			FilesystemLabel: "HARV_LH_DEFAULT",
			Size:            0,
			FS:              "ext4",
		},
	}

	return elementalConfig, nil
}
