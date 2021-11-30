package config

import (
	"fmt"
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

type Network struct {
	Interfaces   []NetworkInterface `json:"interfaces,omitempty"`
	Method       string             `json:"method,omitempty"`
	IP           string             `json:"ip,omitempty"`
	SubnetMask   string             `json:"subnetMask,omitempty"`
	Gateway      string             `json:"gateway,omitempty"`
	DefaultRoute bool               `json:"-"`
	BondOptions  map[string]string  `json:"bondOptions,omitempty"`
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
	Automatic bool               `json:"automatic,omitempty"`
	Mode      string             `json:"mode,omitempty"`
	Networks  map[string]Network `json:"networks,omitempty"`

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
	ForceMBR  bool   `json:"forceMbr,omitempty"` // ForceMBR is not a cOS installer flag

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

type HarvesterConfig struct {
	ServerURL string `json:"serverUrl,omitempty"`
	Token     string `json:"token,omitempty"`

	OS                     `json:"os,omitempty"`
	Install                `json:"install,omitempty"`
	RuntimeVersion         string            `json:"runtimeVersion,omitempty"`
	HarvesterChartVersion  string            `json:"harvesterChartVersion,omitempty"`
	MonitoringChartVersion string            `json:"monitoringChartVersion,omitempty"`
	SystemSettings         map[string]string `json:"systemSettings,omitempty"`
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
