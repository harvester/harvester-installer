package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	SanitizeMask = "***"
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
	SoftMinDiskSizeGiB   = 140
	HardMinDiskSizeGiB   = 60
	MinCosPartSizeGiB    = 25
	NormalCosPartSizeGiB = 50
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

type Install struct {
	Automatic           bool    `json:"automatic,omitempty"`
	Mode                string  `json:"mode,omitempty"`
	ManagementInterface Network `json:"managementInterface,omitempty"`

	Vip       string `json:"vip,omitempty"`
	VipHwAddr string `json:"vipHwAddr,omitempty"`
	VipMode   string `json:"vipMode,omitempty"`

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

	// Following options are not cOS installer flag
	ForceMBR bool   `json:"forceMbr,omitempty"`
	DataDisk string `json:"dataDisk,omitempty"`

	Webhooks []Webhook `json:"webhooks,omitempty"`
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
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`
	WriteFiles        []File   `json:"writeFiles,omitempty"`
	Hostname          string   `json:"hostname,omitempty"`

	Modules        []string          `json:"modules,omitempty"`
	Sysctls        map[string]string `json:"sysctls,omitempty"`
	NTPServers     []string          `json:"ntpServers,omitempty"`
	DNSNameservers []string          `json:"dnsNameservers,omitempty"`
	Wifi           []Wifi            `json:"wifi,omitempty"`
	Password       string            `json:"password,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

type ClusterNetwork struct {
	// by default: false
	Enable      bool              `json:"enable,omitempty"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

type HarvesterConfig struct {
	ServerURL string `json:"serverUrl,omitempty"`
	Token     string `json:"token,omitempty"`

	OS                     `json:"os,omitempty"`
	Install                `json:"install,omitempty"`
	RuntimeVersion         string                    `json:"runtimeVersion,omitempty"`
	RancherVersion         string                    `json:"rancherVersion,omitempty"`
	HarvesterChartVersion  string                    `json:"harvesterChartVersion,omitempty"`
	MonitoringChartVersion string                    `json:"monitoringChartVersion,omitempty"`
	SystemSettings         map[string]string         `json:"systemSettings,omitempty"`
	ClusterNetworks        map[string]ClusterNetwork `json:"clusterNetworks,omitempty"`
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

	if len(labelStrs) > 0 {
		return []string{
			fmt.Sprintf("node-labels=%s", strings.Join(labelStrs, ",")),
		}, nil
	}

	return []string{}, nil
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
