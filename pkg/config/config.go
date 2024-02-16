package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/imdario/mergo"
	yipSchema "github.com/mudler/yip/pkg/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	SchemeVersion = 1
	SanitizeMask  = "***"
)

type NetworkInterface struct {
	Name   string `json:"name,omitempty"`
	HwAddr string `json:"hwAddr,omitempty"`
}

const (
	BondModeBalanceRR    = "balance-rr"
	BondModeActiveBackup = "active-backup"
	BondModeBalnaceXOR   = "balance-xor"
	BondModeBroadcast    = "broadcast"
	BondModeIEEE802_3ad  = "802.3ad"
	BondModeBalanceTLB   = "balance-tlb"
	BondModeBalanceALB   = "balance-alb"
)

const (
	SingleDiskMinSizeGiB   = 250
	MultipleDiskMinSizeGiB = 180
	HardMinDataDiskSizeGiB = 50
	MaxPods                = 200
)

// refer: https://github.com/harvester/harvester/blob/master/pkg/settings/settings.go
func GetSystemSettingsAllowList() []string {
	return []string{
		"additional-ca",
		"api-ui-version",
		"cluster-registration-url",
		"server-version",
		"ui-index",
		"ui-path",
		"ui-source",
		"volume-snapshot-class",
		"backup-target",
		"upgradable-versions",
		"upgrade-checker-enabled",
		"upgrade-checker-url",
		"release-download-url",
		"log-level",
		"ssl-certificates",
		"ssl-parameters",
		"support-bundle-image",
		"support-bundle-namespaces",
		"support-bundle-timeout",
		"default-storage-class",
		"http-proxy",
		"vm-force-reset-policy",
		"overcommit-config",
		"vip-pools",
		"auto-disk-provision-paths",
		"containerd-registry",
		"ntp-servers",
	}
}

type Network struct {
	Interfaces   []NetworkInterface `json:"interfaces,omitempty"`
	Method       string             `json:"method,omitempty"`
	IP           string             `json:"ip,omitempty"`
	SubnetMask   string             `json:"subnetMask,omitempty"`
	Gateway      string             `json:"gateway,omitempty"`
	DefaultRoute bool               `json:"-"`
	BondOptions  map[string]string  `json:"bondOptions,omitempty"`
	MTU          int                `json:"mtu,omitempty"`
	VlanID       int                `json:"vlanId,omitempty"`
}

type NTPSettings struct {
	NTPServers []string `json:"ntpServers,omitempty"`
}

type HTTPBasicAuth struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type Webhook struct {
	Event     string              `json:"event,omitempty"`
	Method    string              `json:"method,omitempty"`
	Headers   map[string][]string `json:"headers,omitempty"`
	URL       string              `json:"url,omitempty"`
	Payload   string              `json:"payload,omitempty"`
	Insecure  bool                `json:"insecure,omitempty"`
	BasicAuth HTTPBasicAuth       `json:"basicAuth,omitempty"`
}

type Addon struct {
	Enabled       bool   `json:"enabled,omitempty"`
	ValuesContent string `json:"valuesContent,omitempty"`
}

type LHDefaultSettings struct {
	// 0 is valid, means not setting CPU resources, use pointer to check if it is set
	GuaranteedEngineManagerCPU  *uint32 `json:"guaranteedEngineManagerCPU,omitempty"`
	GuaranteedReplicaManagerCPU *uint32 `json:"guaranteedReplicaManagerCPU,omitempty"`
}

type LonghornChartValues struct {
	DefaultSettings LHDefaultSettings `json:"defaultSettings,omitempty"`
}

type StorageClass struct {
	// 0 is invalid, will be omitted
	ReplicaCount uint32 `json:"replicaCount,omitempty"`
}

type HarvesterChartValues struct {
	StorageClass     StorageClass        `json:"storageClass,omitempty"`
	Longhorn         LonghornChartValues `json:"longhorn,omitempty"`
	EnableGoCoverDir bool                `json:"enableGoCoverDir,omitempty"`
}

type Install struct {
	Automatic           bool    `json:"automatic,omitempty"`
	Mode                string  `json:"mode,omitempty"`
	ManagementInterface Network `json:"managementInterface,omitempty"`

	Vip       string `json:"vip,omitempty"`
	VipHwAddr string `json:"vipHwAddr,omitempty"`
	VipMode   string `json:"vipMode,omitempty"`

<<<<<<< HEAD
	ForceEFI  bool   `json:"forceEfi,omitempty"`
	Device    string `json:"device,omitempty"`
	ConfigURL string `json:"configUrl,omitempty"`
	Silent    bool   `json:"silent,omitempty"`
	ISOURL    string `json:"isoUrl,omitempty"`
	PowerOff  bool   `json:"powerOff,omitempty"`
	NoFormat  bool   `json:"noFormat,omitempty"`
	Debug     bool   `json:"debug,omitempty"`
	TTY       string `json:"tty,omitempty"`
	ForceGPT  bool   `json:"forceGpt,omitempty"`
=======
	ForceEFI      bool   `json:"forceEfi,omitempty"`
	Device        string `json:"device,omitempty"`
	ConfigURL     string `json:"configUrl,omitempty"`
	Silent        bool   `json:"silent,omitempty"`
	ISOURL        string `json:"isoUrl,omitempty"`
	PowerOff      bool   `json:"powerOff,omitempty"`
	NoFormat      bool   `json:"noFormat,omitempty"`
	Debug         bool   `json:"debug,omitempty"`
	TTY           string `json:"tty,omitempty"`
	ForceGPT      bool   `json:"forceGpt,omitempty"`
	Role          string `json:"role,omitempty"`
	WithNetImages bool   `json:"withNetImages,omitempty"`
	WipeDisks     bool   `json:"wipeDisks,omitempty"`
>>>>>>> 63fd292 (additional installer option to wipe all disks before install)

	// Following options are not cOS installer flag
	ForceMBR bool   `json:"forceMbr,omitempty"`
	DataDisk string `json:"dataDisk,omitempty"`

	Webhooks                []Webhook            `json:"webhooks,omitempty"`
	Addons                  map[string]Addon     `json:"addons,omitempty"`
	Harvester               HarvesterChartValues `json:"harvester,omitempty"`
	RawDiskImagePath        string               `json:"rawDiskImagePath,omitempty"`
	PersistentPartitionSize string               `json:"persistentPartitionSize,omitempty"`
}

type Wifi struct {
	Name       string `json:"name,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type File struct {
	Encoding           string `json:"encoding"`
	Content            string `json:"content"`
	Owner              string `json:"owner"`
	Path               string `json:"path"`
	RawFilePermissions string `json:"permissions"`
}

type OS struct {
	AfterInstallChrootCommands []string `json:"afterInstallChrootCommands,omitempty"`
	SSHAuthorizedKeys          []string `json:"sshAuthorizedKeys,omitempty"`
	WriteFiles                 []File   `json:"writeFiles,omitempty"`
	Hostname                   string   `json:"hostname,omitempty"`

	Modules        []string          `json:"modules,omitempty"`
	Sysctls        map[string]string `json:"sysctls,omitempty"`
	NTPServers     []string          `json:"ntpServers,omitempty"`
	DNSNameservers []string          `json:"dnsNameservers,omitempty"`
	Wifi           []Wifi            `json:"wifi,omitempty"`
	Password       string            `json:"password,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	SSHD           SSHDConfig        `json:"sshd,omitempty"`

	PersistentStatePaths []string `json:"persistentStatePaths,omitempty"`
}

// SSHDConfig is the SSHD configuration for the node
//
//   - SFTP: the switch to enable/disable SFTP
type SSHDConfig struct {
	SFTP bool `json:"sftp,omitempty"`
}

type HarvesterConfig struct {
	// Harvester will use scheme version to determine current version and migrate config to new scheme version
	SchemeVersion          uint32 `json:"schemeVersion,omitempty"`
	ServerURL              string `json:"serverUrl,omitempty"`
	Token                  string `json:"token,omitempty"`
	OS                     `json:"os,omitempty"`
	Install                `json:"install,omitempty"`
	RuntimeVersion         string            `json:"runtimeVersion,omitempty"`
	RancherVersion         string            `json:"rancherVersion,omitempty"`
	HarvesterChartVersion  string            `json:"harvesterChartVersion,omitempty"`
	MonitoringChartVersion string            `json:"monitoringChartVersion,omitempty"`
	SystemSettings         map[string]string `json:"systemSettings,omitempty"`
	LoggingChartVersion    string            `json:"loggingChartVersion,omitempty"`
}

func NewHarvesterConfig() *HarvesterConfig {
	return &HarvesterConfig{}
}

func (c *HarvesterConfig) DeepCopy() (*HarvesterConfig, error) {
	newConf := NewHarvesterConfig()
	if err := mergo.Merge(newConf, c, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("fail to create copy of %T at %p: %s", *c, c, err.Error())
	}
	return newConf, nil
}

func (c *HarvesterConfig) sanitized() (*HarvesterConfig, error) {
	copied, err := c.DeepCopy()
	if err != nil {
		return nil, err
	}
	if copied.Password != "" {
		copied.Password = SanitizeMask
	}
	if copied.Token != "" {
		copied.Token = SanitizeMask
	}
	for i := range copied.Wifi {
		copied.Wifi[i].Passphrase = SanitizeMask
	}
	return copied, nil
}

func (c *HarvesterConfig) String() string {
	s, err := c.sanitized()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%+v", *s)
}

func (c *HarvesterConfig) GetKubeletArgs() ([]string, error) {
	// node-labels=key1=val1,key2=val2
	// max-pods=200 https://github.com/harvester/harvester/issues/2707
	labelStrs := make([]string, 0, len(c.Labels))
	for labelName, labelValue := range c.Labels {
		if errs := validation.IsQualifiedName(labelName); len(errs) > 0 {
			errJoined := strings.Join(errs, ", ")
			return nil, fmt.Errorf("Invalid label name '%s': %s", labelName, errJoined)
		}

		if errs := validation.IsValidLabelValue(labelValue); len(errs) > 0 {
			errJoined := strings.Join(errs, ", ")
			return nil, fmt.Errorf("Invalid label value '%s': %s", labelValue, errJoined)
		}
		labelStrs = append(labelStrs, fmt.Sprintf("%s=%s", labelName, labelValue))
	}

	var args = []string{
		fmt.Sprintf("max-pods=%d", MaxPods),
	}

	if len(labelStrs) > 0 {
		args = append(args,
			fmt.Sprintf("node-labels=%s", strings.Join(labelStrs, ",")),
		)
	}

	return args, nil
}

func (c HarvesterConfig) ShouldCreateDataPartitionOnOsDisk() bool {
	// DataDisk is empty means only using the OS disk, and most of the time we should create data
	// partition on OS disk, unless when ForceMBR=true then we should not create data partition.
	return c.DataDisk == "" && !c.ForceMBR
}

func (c HarvesterConfig) ShouldMountDataPartition() bool {
	// With ForceMBR=true and no DataDisk assigned (Using the OS disk), no data partition/disk will
	// be created, so no need to mount the data disk/partition
	if c.ForceMBR && c.DataDisk == "" {
		return false
	}

	return true
}

func (c *HarvesterConfig) Merge(other HarvesterConfig) error {
	if err := mergo.Merge(c, other, mergo.WithAppendSlice); err != nil {
		return err
	}

	return nil
}

func (n *NetworkInterface) FindNetworkInterfaceNameAndHwAddr() error {
	if err := n.FindNetworkInterfaceName(); err != nil {
		return err
	}

	if err := n.FindNetworkInterfaceHwAddr(); err != nil {
		return err
	}

	// Default, there is no Name or HwAddress, do nothing. Let validation capture it
	return nil
}

// FindNetworkInterfaceName uses MAC address to lookup interface name
func (n *NetworkInterface) FindNetworkInterfaceName() error {
	if n.Name != "" {
		return nil
	}

	if n.Name == "" && n.HwAddr != "" {
		hwAddr, err := net.ParseMAC(n.HwAddr)
		if err != nil {
			return err
		}

		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		for _, iface := range interfaces {
			if iface.HardwareAddr.String() == hwAddr.String() {
				n.Name = iface.Name
				return nil
			}
		}

		return fmt.Errorf("no interface matching hardware address %s found", n.HwAddr)
	}

	// Default, there is no Name or HwAddress, do nothing. Let validation capture it
	return nil

}

// FindNetworkInterfaceHwAddr uses device name to lookup hardware address
func (n *NetworkInterface) FindNetworkInterfaceHwAddr() error {
	if n.HwAddr != "" {
		return nil
	}

	if n.Name != "" && n.HwAddr == "" {
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		for _, iface := range interfaces {
			if iface.Name == n.Name {
				n.HwAddr = iface.HardwareAddr.String()
				return nil
			}
		}

		return fmt.Errorf("no interface matching name %s found", n.Name)
	}

	// Default, there is no Name or HwAddress, do nothing. Let validation capture it
	return nil
}

func GenerateRancherdConfig(config *HarvesterConfig) (*yipSchema.YipConfig, error) {

	runtimeConfig := yipSchema.Stage{
		Users:            make(map[string]yipSchema.User),
		TimeSyncd:        make(map[string]string),
		SSHKeys:          make(map[string][]string),
		Sysctl:           make(map[string]string),
		Environment:      make(map[string]string),
		SystemdFirstBoot: make(map[string]string),
	}

	runtimeConfig.Hostname = config.OS.Hostname
	if len(config.OS.NTPServers) > 0 {
		runtimeConfig.TimeSyncd["NTP"] = strings.Join(config.OS.NTPServers, " ")
		runtimeConfig.Systemctl.Enable = append(runtimeConfig.Systemctl.Enable, ntpdService)
		runtimeConfig.Systemctl.Enable = append(runtimeConfig.Systemctl.Enable, timeWaitSyncService)
	}
	if len(config.OS.DNSNameservers) > 0 {
		runtimeConfig.Commands = append(runtimeConfig.Commands, getAddStaticDNSServersCmd(config.OS.DNSNameservers))
	}
	err := initRancherdStage(config, &runtimeConfig)
	if err != nil {
		return nil, err
	}

	if err := UpdateWifiConfig(&runtimeConfig, config.OS.Wifi, true); err != nil {
		return nil, err
	}

	if _, err := UpdateManagementInterfaceConfig(&runtimeConfig, config.ManagementInterface, true); err != nil {
		return nil, err
	}

	runtimeConfig.SSHKeys[cosLoginUser] = config.OS.SSHAuthorizedKeys
	runtimeConfig.Users[cosLoginUser] = yipSchema.User{
		PasswordHash: config.OS.Password,
	}

	conf := &yipSchema.YipConfig{
		Name: "RancherD Configuration",
		Stages: map[string][]yipSchema.Stage{
			"live": {
				runtimeConfig,
			},
		},
	}

	return conf, nil
}
