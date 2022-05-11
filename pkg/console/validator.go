package console

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester-installer/pkg/config"
)

const validTokenChars = "[a-zA-Z0-9 !\"#$%&'()*+,-./:;<=>?@^_`{|}~[\\]\\\\]"

var (
	ErrMsgModeCreateContainsServerURL   = fmt.Sprintf("ServerURL need to be empty in %s mode", config.ModeCreate)
	ErrMsgModeJoinServerURLNotSpecified = fmt.Sprintf("ServerURL can't empty in %s mode", config.ModeJoin)
	ErrMsgModeUnknown                   = "unknown mode"
	ErrMsgTokenNotSpecified             = "token not specified"
	ErrMsgISOURLNotSpecified            = "iso_url is required in automatic installation"

	ErrMsgMgmtInterfaceNotSpecified    = "no management interface specified"
	ErrMsgMgmtInterfaceInvalidMethod   = "management network must configure with either static or DHCP method"
	ErrMsgMgmtInterfaceStaticNoDNS     = "DNS servers are required for static IP address"
	ErrMsgInterfaceNotSpecified        = "no interface specified"
	ErrMsgInterfaceNotSpecifiedForMgmt = "no interface specified for management network"
	ErrMsgInterfaceNotFound            = "interface not found"
	ErrMsgInterfaceIsLoop              = "interface is a loopback interface"
	ErrMsgDeviceNotSpecified           = "no device specified"
	ErrMsgDeviceNotFound               = "device not found"
	ErrMsgDeviceTooSmall               = fmt.Sprintf("device size too small. At least %dG is required", config.HardMinDiskSizeGiB)
	ErrMsgNoCredentials                = "no SSH authorized keys or passwords are set"
	ErrMsgForceMBROnLargeDisk          = "disk size too large for MBR partitioning table"
	ErrMsgForceMBROnUEFI               = "cannot force MBR on UEFI system"

	ErrMsgNetworkMethodUnknown = "unknown network method"

	ErrMsgSystemSettingsUnknown = "unknown system settings: %s"
)

type ValidatorInterface interface {
	Validate(cfg *config.HarvesterConfig) error
}

type ConfigValidator struct {
}

func prettyError(errMsg string, value string) error {
	return errors.Errorf("%s: %s", errMsg, value)
}

func checkInterface(iface config.NetworkInterface) error {

	err := iface.FindNetworkInterfaceName()
	if err != nil {
		return err
	}

	// For now we only accept interface name but not device specifier.
	if iface.Name == "" {
		return errors.New(ErrMsgInterfaceNotSpecified)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	checkFlags := func(flags net.Flags, name string) error {
		if flags&net.FlagLoopback != 0 {
			return prettyError(ErrMsgInterfaceIsLoop, name)
		}
		return nil
	}

	for _, i := range ifaces {
		if i.Name == iface.Name {
			return checkFlags(i.Flags, iface.Name)
		}
	}
	return prettyError(ErrMsgInterfaceNotFound, iface.Name)
}

func checkDevice(device string) error {
	if device == "" {
		return errors.New(ErrMsgDeviceNotSpecified)
	}

	fileInfo, err := os.Lstat(device)
	if err != nil {
		return err
	}

	targetDevice := device
	// Support using path like `/dev/disks/by-id/xxx`
	if fileInfo.Mode()&fs.ModeSymlink != 0 {
		targetDevice, err = filepath.EvalSymlinks(device)
		if err != nil {
			return err
		}
	}

	options, err := getDiskOptions()
	if err != nil {
		return err
	}

	deviceFound := false
	for _, option := range options {
		if targetDevice == option.Value {
			deviceFound = true
			break
		}
	}
	if !deviceFound {
		return prettyError(ErrMsgDeviceNotFound, device)
	}

	if err := validateDiskSize(device); err != nil {
		return prettyError(ErrMsgDeviceTooSmall, device)
	}

	return nil
}

func checkStaticRequiredString(field, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("must specify %s in static method", field)
	}
	return nil
}

func checkDomain(domain string) error {
	if errs := validation.IsDNS1123Subdomain(domain); len(errs) > 0 {
		return fmt.Errorf("%s is not a valid domain", domain)
	}
	return nil
}

func checkIP(addr string) error {
	if ip := net.ParseIP(addr); ip == nil || ip.To4() == nil {
		return fmt.Errorf("%s is not a valid IP address", addr)
	}
	return nil
}

func checkHwAddr(hwAddr string) error {
	if _, err := net.ParseMAC(hwAddr); err != nil {
		return fmt.Errorf("%s is an invalid hardware address, error: %w", hwAddr, err)
	}

	return nil
}

func checkIPList(ipList []string) error {
	for _, ip := range ipList {
		if err := checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func checkNetworks(network config.Network, dnsServers []string) error {
	if len(network.Interfaces) == 0 {
		return errors.New(ErrMsgInterfaceNotSpecifiedForMgmt)
	}
	method := network.Method
	if method != config.NetworkMethodDHCP && method != config.NetworkMethodStatic {
		return errors.New(ErrMsgMgmtInterfaceInvalidMethod)
	}
	if method == config.NetworkMethodStatic && len(dnsServers) == 0 {
		return errors.New(ErrMsgMgmtInterfaceStaticNoDNS)
	}

	for _, iface := range network.Interfaces {
		if err := checkInterface(iface); err != nil {
			return err
		}
	}

	// TODO check VLAN ID in 0-4094 (0 is unset)

	switch network.Method {
	case config.NetworkMethodDHCP, config.NetworkMethodNone, "":
		return nil
	case config.NetworkMethodStatic:
		if err := checkStaticRequiredString("ip", network.IP); err != nil {
			return err
		}
		if err := checkIP(network.IP); err != nil {
			return err
		}
		if err := checkStaticRequiredString("subnetMask", network.SubnetMask); err != nil {
			return err
		}
		if err := checkIP(network.SubnetMask); err != nil {
			return err
		}
		if err := checkStaticRequiredString("gateway", network.Gateway); err != nil {
			return err
		}
		if err := checkIP(network.Gateway); err != nil {
			return err
		}
	default:
		return prettyError(ErrMsgNetworkMethodUnknown, network.Method)
	}

	return nil
}

func checkVip(vip, vipHwAddr, vipMode string) error {
	if err := checkIP(vip); err != nil {
		return err
	}

	switch vipMode {
	case config.NetworkMethodDHCP:
		if err := checkHwAddr(vipHwAddr); err != nil {
			return err
		}
	case config.NetworkMethodStatic, config.NetworkMethodNone:
		return nil
	default:
		return prettyError(ErrMsgNetworkMethodUnknown, vipMode)
	}

	return nil
}

func checkForceMBR(device string) error {
	diskTooLargeForMBR, err := diskExceedsMBRLimit(device)
	if err != nil {
		return err
	}
	if diskTooLargeForMBR {
		return prettyError(ErrMsgForceMBROnLargeDisk, device)
	}

	if !systemIsBIOS() {
		return prettyError(ErrMsgForceMBROnUEFI, "UEFI")
	}

	return nil
}

func checkToken(token string) error {
	pattern := regexp.MustCompile("^" + validTokenChars + "+$")
	if !pattern.MatchString(token) {
		return errors.Errorf(
			"Invalid token. Must be alphanumeric and OWASP special password characters. Regexp: %v",
			pattern,
		)
	}

	return nil
}

func checkSystemSettings(systemSettings map[string]string) error {
	if systemSettings == nil {
		return nil
	}

	allowList := config.GetSystemSettingsAllowList()
	for systemSetting := range systemSettings {
		isValid := false
		for _, allowSystemSetting := range allowList {
			if systemSetting == allowSystemSetting {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.Errorf(ErrMsgSystemSettingsUnknown, systemSetting)
		}
	}
	return nil
}

func (v ConfigValidator) Validate(cfg *config.HarvesterConfig) error {
	// check hostname
	// ref: https://github.com/kubernetes/kubernetes/blob/b15f788d29df34337fedc4d75efe5580c191cbf3/pkg/apis/core/validation/validation.go#L242-L245
	if errs := validation.IsDNS1123Subdomain(cfg.OS.Hostname); len(errs) > 0 {
		// TODO: show regexp for validation to users
		return errors.Errorf("Invalid hostname. A lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'.")
	}

	if err := checkDevice(cfg.Install.Device); err != nil {
		return err
	}

	if cfg.Install.DataDisk != "" {
		if err := checkDevice(cfg.Install.DataDisk); err != nil {
			return err
		}
	}

	if cfg.ForceMBR {
		if err := checkForceMBR(cfg.Install.Device); err != nil {
			return err
		}
	}

	if err := checkNetworks(cfg.Install.ManagementInterface, cfg.OS.DNSNameservers); err != nil {
		return err
	}

	if cfg.Install.Mode == config.ModeCreate {
		if err := checkVip(cfg.Vip, cfg.VipHwAddr, cfg.VipMode); err != nil {
			return err
		}

		if err := checkSystemSettings(cfg.SystemSettings); err != nil {
			return err
		}
	}

	if _, err := cfg.GetKubeletArgs(); err != nil {
		return err
	}

	if err := checkToken(cfg.Token); err != nil {
		return err
	}

	return nil
}

func commonCheck(cfg *config.HarvesterConfig) error {
	// modes
	switch mode := cfg.Install.Mode; mode {
	case config.ModeUpgrade:
		return nil
	case config.ModeCreate:
		if cfg.ServerURL != "" {
			return errors.New(ErrMsgModeCreateContainsServerURL)
		}
	case config.ModeJoin:
		if cfg.ServerURL == "" {
			return errors.New(ErrMsgModeJoinServerURLNotSpecified)
		}
	default:
		return prettyError(ErrMsgModeUnknown, mode)
	}

	if cfg.Install.Automatic && cfg.Install.ISOURL == "" {
		return errors.New(ErrMsgISOURLNotSpecified)
	}

	if cfg.Token == "" {
		return errors.New(ErrMsgTokenNotSpecified)
	}

	if len(cfg.SSHAuthorizedKeys) == 0 && cfg.Password == "" {
		return errors.New(ErrMsgNoCredentials)
	}
	return nil
}

func validateConfig(v ValidatorInterface, cfg *config.HarvesterConfig) error {
	logrus.Debug("Validating config: ", cfg)
	if err := commonCheck(cfg); err != nil {
		return err
	}
	return v.Validate(cfg)
}
