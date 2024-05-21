package console

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	yipSchema "github.com/mudler/yip/pkg/schema"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

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
	var nics []netlink.Link

	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	for _, l := range links {
		if l.Type() == "device" && l.Attrs().EncapType != "loopback" {
			nics = append(nics, l)
		}
	}

	return nics, nil
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
