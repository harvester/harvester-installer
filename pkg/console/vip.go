package console

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

const tempMacvlanName = "macvlan-0"

type vipAddr struct {
	hwAddr   string
	ipv4Addr string
}

func createMacvlan(name string) (netlink.Link, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", name)
	}
	macvlan := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        tempMacvlanName,
			ParentIndex: l.Attrs().Index,
		},
	}

	if err = netlink.LinkAdd(macvlan); err != nil {
		return nil, errors.Wrapf(err, "failed to add %s", tempMacvlanName)
	}
	if err = netlink.LinkSetUp(macvlan); err != nil {
		return nil, errors.Wrapf(err, "failed to set %s up", tempMacvlanName)
	}

	return netlink.LinkByName(tempMacvlanName)
}

func deleteMacvlan(l netlink.Link) error {
	// It's necessary to set macvlan down at first to notify the wicked to clear the related files automatically.
	if err := netlink.LinkSetDown(l); err != nil {
		return errors.Wrapf(err, "failed to set %s down", err)
	}
	if err := netlink.LinkDel(l); err != nil {
		return errors.Wrapf(err, "failed to del %s", tempMacvlanName)
	}

	return nil
}

func getVipThroughDHCP(iface string) (*vipAddr, error) {
	l, err := createMacvlan(iface)
	if err != nil {
		return nil, err
	}

	ip, err := getIPThroughDHCP(l)
	if err != nil {
		return nil, err
	}

	if err := deleteMacvlan(l); err != nil {
		return nil, err
	}

	return &vipAddr{
		hwAddr: l.Attrs().HardwareAddr.String(),
		ipv4Addr: ip,
	}, nil
}

func getIPThroughDHCP(l netlink.Link) (string, error) {
	out, err := exec.Command("/bin/sh", "-c", "/usr/lib/wicked/bin/wickedd-dhcp4 --test "+l.Attrs().Name).CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, "failed to get IP through DHCP")
	}

	return parseIPFromWickedOutput(out)
}

func parseIPFromWickedOutput(out []byte) (string, error) {
	lines := strings.Split(string(out), "\n")

	var ip string
	for _, line := range lines {
		words := strings.FieldsFunc(line, func(c rune) bool {
			if c == '=' || c == '\'' {
				return true
			}
			return false
		})
		if len(words) < 2 {
			continue
		}
		if words[0] == "IPADDR" {
			ip = strings.Split(words[1], "/")[0]
			break
		}
	}

	if ip == "" {
		return ip, fmt.Errorf("IPv4 address can not be empty string")
	}

	return ip, nil
}
