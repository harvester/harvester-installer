package console

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester-installer/pkg/config"
)

var (
	ErrMsgModeCreateContainsServerURL   = fmt.Sprintf("ServerURL need to be empty in %s mode", config.ModeCreate)
	ErrMsgModeJoinServerURLNotSpecified = fmt.Sprintf("ServerURL can't empty in %s mode", config.ModeJoin)
	ErrMsgModeUnknown                   = "unknown mode"
	ErrMsgTokenNotSpecified             = "token not specified"

	ErrMsgMgmtInterfaceNotSpecified    = "no management interface specified"
	ErrMsgMgmtInterfaceInvalidMethod   = "management network must configure with either static or DHCP method"
	ErrMsgInterfaceNotSpecified        = "no interface specified"
	ErrMsgInterfaceNotSpecifiedForMgmt = "no interface specified for management network"
	ErrMsgInterfaceNotFound            = "interface not found"
	ErrMsgInterfaceIsLoop              = "interface is a loopback interface"
	ErrMsgDeviceNotSpecified           = "no device specified"
	ErrMsgDeviceNotFound               = "device not found"
	ErrMsgNoCredentials                = "no SSH authorized keys or passwords are set"

	ErrMsgNetworkMethodUnknown = "unknown network method"
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
	options, err := getDiskOptions()
	if err != nil {
		return err
	}
	for _, option := range options {
		if device == option.Value {
			return nil
		}
	}
	return prettyError(ErrMsgDeviceNotFound, device)
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

func checkNetworks(networks map[string]config.Network) error {
	if len(networks) == 0 {
		return errors.New(ErrMsgMgmtInterfaceNotSpecified)
	}

	if mgmtNetwork, ok := networks[config.MgmtInterfaceName]; !ok {
		return errors.New(ErrMsgMgmtInterfaceNotSpecified)
	} else {
		if len(mgmtNetwork.Interfaces) == 0 {
			return errors.New(ErrMsgInterfaceNotSpecifiedForMgmt)
		}
		method := mgmtNetwork.Method
		if method != config.NetworkMethodDHCP && method != config.NetworkMethodStatic {
			return errors.New(ErrMsgMgmtInterfaceInvalidMethod)
		}
	}

	for _, network := range networks {
		for _, iface := range network.Interfaces {
			if err := checkInterface(iface); err != nil {
				return err
			}
		}
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

	if err := checkNetworks(cfg.Install.Networks); err != nil {
		return err
	}

	if cfg.Install.Mode == config.ModeCreate {
		if err := checkVip(cfg.Vip, cfg.VipHwAddr, cfg.VipMode); err != nil {
			return err
		}
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
