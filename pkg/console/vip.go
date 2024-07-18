package console

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	gocommon "github.com/harvester/go-common"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const tempMacvlanPrefix = "macvlan-"
const DhcpOption57DefaultValue = 576

type vipAddr struct {
	hwAddr   string
	ipv4Addr string
}

func createMacvlan(name string) (netlink.Link, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", name)
	}
	randNum, err := gocommon.GenRandNumber(100)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate random number")
	}
	macvlanName := tempMacvlanPrefix + strconv.Itoa(int(randNum))
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

func getVipThroughDHCP(iface string) (*vipAddr, error) {
	l, err := createMacvlan(iface)
	if err != nil {
		return nil, err
	}

	ip, err := getIPThroughDHCP(l.Attrs().Name)
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

func getIPThroughDHCP(iface string) (net.IP, error) {
	// original solution, with option57 1500
	ip, err1 := getIPThroughDHCPOriginal(iface)
	if err1 == nil {
		return ip, nil
	}

	// some DHCP servers may fail with the above option, try again with the default value
	ip, err2 := getIPThroughDHCPWithOption57(iface, DhcpOption57DefaultValue)
	if err2 == nil {
		return ip, nil
	}
	logrus.Infof("dhcp from %s option57 value max %s default %s", iface, err1.Error(), err2.Error())
	return nil, fmt.Errorf("dhcp from %s option57 value max %w default %w", iface, err1, err2)
}

// original code
func getIPThroughDHCPOriginal(iface string) (net.IP, error) {
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

func getIPThroughDHCPWithOption57(iface string, msz uint16) (net.IP, error) {
	broadcast, err := nclient4.New(iface)
	if err != nil {
		return nil, err
	}
	defer broadcast.Close()

	// if this param is not set, use the original default value
	if msz == 0 || msz > nclient4.MaxMessageSize {
		msz = nclient4.MaxMessageSize
	} else if msz < DhcpOption57DefaultValue {
		msz = DhcpOption57DefaultValue
	}

	lease, err := request(broadcast, context.TODO(), uint16(msz))
	if err != nil {
		return nil, err
	}

	logrus.Info(lease)

	return lease.Offer.YourIPAddr, nil
}

// below code is copied from
// https://github.com/insomniacslk/dhcp/blob/master/dhcpv4/nclient4/client.go#L38
// but use a smaller OptMaxMessageSize

func request(c *nclient4.Client, ctx context.Context, msz uint16, modifiers ...dhcpv4.Modifier) (lease *nclient4.Lease, err error) {

	offer, err := discoverOffer(c, ctx, msz, modifiers...)
	if err != nil {
		err = fmt.Errorf("unable to receive an offer: %w", err)
		return
	}
	return requestFromOffer(c, ctx, msz, offer, modifiers...)
}

// DiscoverOffer sends a DHCPDiscover message and returns the first valid offer
// received.
func discoverOffer(c *nclient4.Client, ctx context.Context, msz uint16, modifiers ...dhcpv4.Modifier) (offer *dhcpv4.DHCPv4, err error) {
	// RFC 2131, Section 4.4.1, Table 5 details what a DISCOVER packet should
	// contain.
	discover, err := dhcpv4.NewDiscovery(c.InterfaceAddr(), dhcpv4.PrependModifiers(modifiers,
		dhcpv4.WithOption(dhcpv4.OptMaxMessageSize(msz)))...)
	if err != nil {
		return nil, fmt.Errorf("unable to create a DHCP discovery request: %w", err)
	}

	offer, err = c.SendAndRead(ctx, c.RemoteAddr(), discover, nclient4.IsMessageType(dhcpv4.MessageTypeOffer))
	if err != nil {
		return nil, fmt.Errorf("got an error while the DHCP discovery request: %w", err)
	}
	return offer, nil
}

// RequestFromOffer sends a Request message and waits for an response.
// It assumes the SELECTING state by default, see Section 4.3.2 in RFC 2131 for more details.
func requestFromOffer(c *nclient4.Client, ctx context.Context, msz uint16, offer *dhcpv4.DHCPv4, modifiers ...dhcpv4.Modifier) (*nclient4.Lease, error) {
	// TODO(chrisko): should this be unicast to the server?
	request, err := dhcpv4.NewRequestFromOffer(offer, dhcpv4.PrependModifiers(modifiers,
		dhcpv4.WithOption(dhcpv4.OptMaxMessageSize(msz)))...)
	if err != nil {
		return nil, fmt.Errorf("unable to create a DHCP request: %w", err)
	}

	// Servers are supposed to only respond to Requests containing their server identifier,
	// but sometimes non-compliant servers respond anyway.
	// Clients are not required to validate this field, but servers are required to
	// include the server identifier in their Offer per RFC 2131 Section 4.3.1 Table 3.
	response, err := c.SendAndRead(ctx, c.RemoteAddr(), request, nclient4.IsAll(
		nclient4.IsCorrectServer(offer.ServerIdentifier()),
		nclient4.IsMessageType(dhcpv4.MessageTypeAck, dhcpv4.MessageTypeNak)))
	if err != nil {
		return nil, fmt.Errorf("got an error while processing the DHCP request: %w", err)
	}
	if response.MessageType() == dhcpv4.MessageTypeNak {
		return nil, &nclient4.ErrNak{
			Offer: offer,
			Nak:   response,
		}
	}
	lease := &nclient4.Lease{}
	lease.ACK = response
	lease.Offer = offer
	lease.CreationTime = time.Now()
	return lease, nil
}
