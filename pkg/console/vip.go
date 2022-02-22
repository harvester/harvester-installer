package console

import (
	"context"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const tempMacvlanPrefix = "macvlan-"

type vipAddr struct {
	hwAddr   string
	ipv4Addr string
}

func createMacvlan(name string) (netlink.Link, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", name)
	}
	rand.Seed(time.Now().UnixNano())
	macvlanName := tempMacvlanPrefix + strconv.Itoa(rand.Intn(100))
	macvlan := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        macvlanName,
			ParentIndex: l.Attrs().Index,
		},
	}

	if err = netlink.LinkAdd(macvlan); err != nil {
		return nil, errors.Wrapf(err, "failed to add %s", macvlanName)
	}
	if err = netlink.LinkSetUp(macvlan); err != nil {
		return nil, errors.Wrapf(err, "failed to set %s up", macvlanName)
	}

	return netlink.LinkByName(macvlanName)
}

func deleteMacvlan(l netlink.Link) error {
	// It's necessary to set macvlan down at first to notify the wicked to clear the related files automatically.
	if err := netlink.LinkSetDown(l); err != nil {
		return errors.Wrapf(err, "failed to set %s down", err)
	}
	if err := netlink.LinkDel(l); err != nil {
		return errors.Wrapf(err, "failed to del %s", l.Attrs().Name)
	}

	return nil
}

func getVipThroughDHCP(iface string, cfg config.Network) (*vipAddr, error) {
	vlanIface := cfg.GetVlanInterfaceName()
	if vlanIface != "" {
		iface = vlanIface
	}

	l, err := createMacvlan(iface)
	if err != nil {
		return nil, err
	}

	// The MACVLAN interface is created on-top of the VLAN interface (if required),
	// so passing an empty Network struct to getIPThroughDHCP to prevent it uses
	// the VLAN interface to request for DHCP IP
	ip, err := getIPThroughDHCP(l.Attrs().Name, config.Network{})
	if err != nil {
		return nil, err
	}

	if err := deleteMacvlan(l); err != nil {
		return nil, err
	}

	return &vipAddr{
		hwAddr:   l.Attrs().HardwareAddr.String(),
		ipv4Addr: ip.String(),
	}, nil
}

// getIPThroughDHCP tries to get a DHCP IP using the given interface and the
// Network struct of the interface. We need the Network struct because we might
// have to use the VLAN interface to send out the DHCP request.
func getIPThroughDHCP(iface string, cfg config.Network) (net.IP, error) {
	vlanIface := cfg.GetVlanInterfaceName()
	if vlanIface != "" {
		iface = vlanIface
	}

	broadcast, err := nclient4.New(iface)
	if err != nil {
		return nil, err
	}
	defer broadcast.Close()

	lease, err := broadcast.Request(context.TODO())
	if err != nil {
		return nil, err
	}

	logrus.Info(lease)

	return lease.Offer.YourIPAddr, nil
}
