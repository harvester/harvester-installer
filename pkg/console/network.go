package console

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"syscall"

	yipSchema "github.com/rancher/yip/pkg/schema"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/dell/goiscsi"

	"github.com/harvester/harvester-installer/pkg/config"
)

func checkDefaultRoute() (bool, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		logrus.Errorf("Failed to list routes: %s", err.Error())
		return false, err
	}

	defaultRouteExists := false
	for _, route := range routes {
		if route.Dst == nil {
			defaultRouteExists = true
			break
		}
	}

	return defaultRouteExists, nil
}

func applyNetworks(network config.Network, hostname string) ([]byte, error) {
	if err := config.RestoreOriginalNetworkConfig(); err != nil {
		return nil, err
	}
	if err := config.SaveOriginalNetworkConfig(); err != nil {
		return nil, err
	}

	// If called without a hostname set, we enable setting hostname via the
	// DHCP server, in case the DHCP server is configured to give us a
	// hostname we can use by default.
	//
	// If we move the network interface page of the installer so it's before
	// the hostname page, this function will activate once the management
	// NIC is configured, and if the DHCP server is configured correctly,
	// the system hostname will be set to the one provided by the server.
	// Later, on the hostname page, we can default the hostname field to
	// the current system hostname.

	dhclientSetHostname := "no"
	if hostname == "" {
		dhclientSetHostname = "yes"
	}
	output, err := exec.Command("sed", "-i",
		fmt.Sprintf(`s/^DHCLIENT_SET_HOSTNAME=.*/DHCLIENT_SET_HOSTNAME="%s"/`, dhclientSetHostname),
		"/etc/sysconfig/network/dhcp").CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return output, err
	}

	conf := &yipSchema.YipConfig{
		Name: "Network Configuration",
		Stages: map[string][]yipSchema.Stage{
			"live": {
				yipSchema.Stage{Hostname: hostname}, // Ensure hostname updated before configuring network
				yipSchema.Stage{},
			},
		},
	}
	_, err = config.UpdateManagementInterfaceConfig(&conf.Stages["live"][1], network, true)
	if err != nil {
		return nil, err
	}

	tempFile, err := ioutil.TempFile("/tmp", "live.XXXXXXXX")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	bytes, err := yaml.Marshal(conf)
	if err != nil {
		return nil, err
	}
	if _, err := tempFile.Write(bytes); err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())

	cmd := exec.Command("/usr/bin/yip", "-s", "live", tempFile.Name())
	cmd.Env = os.Environ()
	bytes, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Error(err, string(bytes))
		return bytes, err
	}
	// Restore Down NIC to up
	if err := upAllLinks(); err != nil {
		logrus.Errorf("failed to bring all link up: %s", err.Error())
	}
	return bytes, err
}

func upAllLinks() error {
	nics, err := getNICs()
	if err != nil {
		return err
	}

	for _, nic := range nics {
		if err := netlink.LinkSetUp(nic); err != nil {
			return err
		}
	}
	return nil
}

func getNICs() ([]netlink.Link, error) {
	var nics, vlanNics []netlink.Link

	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	for _, l := range links {
		if l.Type() == "device" && l.Attrs().EncapType != "loopback" {
			nics = append(nics, l)
		}
		if l.Type() == "vlan" {
			vlanNics = append(vlanNics, l)
		}
	}

	iscsi := goiscsi.NewLinuxISCSI(nil)
	sessions, err := iscsi.GetSessions()
	if err != nil {
		return nil, fmt.Errorf("error querying iscsi sessions: %v", err)
	}

	// no iscsi sessions detected so no additional filtering based on usage for iscsi device
	// access is needed and we can break here
	if len(sessions) == 0 {
		return nics, nil
	}

	return filterISCSIInterfaces(nics, vlanNics, sessions)
}

func getNICState(name string) int {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return NICStateNotFound
	}
	up := link.Attrs().RawFlags&unix.IFF_UP != 0
	lowerUp := link.Attrs().RawFlags&unix.IFF_LOWER_UP != 0
	if !up {
		return NICStateDown
	}
	if !lowerUp {
		return NICStateLowerDown
	}
	return NICStateUP
}

type networkHardwareInfo struct {
	Name        string `json:"logicalname"`
	Vendor      string `json:"vendor"`
	Product     string `json:"product"`
	Description string `json:"description"`
}

func listNetworkHardware() (map[string]networkHardwareInfo, error) {
	out, err := exec.Command("/bin/sh", "-c", "lshw -c network -json").CombinedOutput()
	if err != nil {
		return nil, err
	}

	m := make(map[string]networkHardwareInfo)
	var networkHardwareList []networkHardwareInfo
	if err := json.Unmarshal(out, &networkHardwareList); err != nil {
		return nil, err
	}

	for _, networkHardware := range networkHardwareList {
		m[networkHardware.Name] = networkHardware
	}

	return m, nil
}

func getManagementInterfaceName(mgmtInterface config.Network) string {
	mgmtName := config.MgmtInterfaceName
	vlanID := mgmtInterface.VlanID
	if vlanID >= 2 && vlanID <= 4094 {
		mgmtName = fmt.Sprintf("%s.%d", mgmtName, vlanID)
	}
	return mgmtName
}

// filterISCSIInterfaces will query the host to identify iscsi sessions, and skip interfaces
// used by the existing iscsi session.
func filterISCSIInterfaces(nics, vlanNics []netlink.Link, sessions []goiscsi.ISCSISession) ([]netlink.Link, error) {
	hwDeviceMap := make(map[string]netlink.Link)
	for _, v := range nics {
		hwDeviceMap[v.Attrs().HardwareAddr.String()] = v
	}

	// temporary sessionMap to make it easy to correlate interface addressses with session ip address
	// should speed up identification of interfaces in use with iscsi sessions
	sessionMap := make(map[string]string)
	for _, session := range sessions {
		sessionMap[session.IfaceIPaddress] = ""
	}

	if err := filterNICSBySession(hwDeviceMap, nics, sessionMap); err != nil {
		return nil, err
	}

	if err := filterNICSBySession(hwDeviceMap, vlanNics, sessionMap); err != nil {
		return nil, err
	}
	logrus.Debugf("identified following iscsi sessions: %v", sessionMap)
	// we need to filter the filteredNics to also isolate parent nics if a vlan if is in use
	returnedNics := make([]netlink.Link, 0, len(hwDeviceMap))
	for _, v := range hwDeviceMap {
		returnedNics = append(returnedNics, v)
	}
	return returnedNics, nil
}

func filterNICSBySession(hwDeviceMap map[string]netlink.Link, links []netlink.Link, sessionMap map[string]string) error {
	for _, link := range links {
		logrus.Debugf("checking if link %s is in use", link.Attrs().Name)
		if getNICState(link.Attrs().Name) == NICStateUP {
			iface, err := net.InterfaceByName(link.Attrs().Name)
			if err != nil {
				return fmt.Errorf("error fetching interface details: %v", err)
			}

			addresses, err := iface.Addrs()
			if err != nil {
				return fmt.Errorf("error fetching addresses from interface: %v", err)
			}

			for _, address := range addresses {
				// interface addresses are in cidr format, and need to be converted before comparison
				// since iscsi session contains just the ip address
				ipAddress, _, err := net.ParseCIDR(address.String())
				if err != nil {
					return fmt.Errorf("error parsing ip address: %v", err)
				}
				if _, ok := sessionMap[ipAddress.String()]; ok {
					logrus.Debugf("filtering interface %s", link.Attrs().Name)
					delete(hwDeviceMap, link.Attrs().HardwareAddr.String())
					break //device is already removed, no point checking for other addresses
				}
			}
		}
	}
	return nil
}
