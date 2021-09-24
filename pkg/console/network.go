package console

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"

	yipSchema "github.com/mudler/yip/pkg/schema"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v2"

	"github.com/harvester/harvester-installer/pkg/config"
)

func applyNetworks(networks map[string]config.Network) ([]byte, error) {
	if err := config.RestoreOriginalNetworkConfig(); err != nil {
		return nil, err
	}
	if err := config.SaveOriginalNetworkConfig(); err != nil {
		return nil, err
	}

	conf := &yipSchema.YipConfig{
		Name: "Network Configuration",
		Stages: map[string][]yipSchema.Stage{
			"live": {
				yipSchema.Stage{},
			},
		},
	}
	err := config.UpdateNetworkConfig(&conf.Stages["live"][0], networks, true)
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
	// Restore Down NIC to up
	if err := upAllLinks(); err != nil {
		logrus.Errorf("failed to bring all link up: %s", err.Error())
	}
	return bytes, err
}

func upAllLinks() error {
	ifaces, err := getNetworkInterfaces()
	if err != nil {
		return err
	}
	for _, i := range ifaces {
		if getNICState(i.Name) != NICStateUP {
			cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("ip link set %s up", i.Name))
			cmd.Env = os.Environ()
			_, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("cannot bring NIC %s up: %s", i.Name, err.Error())
			}
		}
	}
	return nil
}

func getNetworkInterfaces() ([]net.Interface, error) {
	var interfaces = []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		link, err := netlink.LinkByName(i.Name)
		if err != nil {
			return nil, err
		}
		if link.Type() != "device" {
			continue
		}
		interfaces = append(interfaces, i)
	}
	return interfaces, nil
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
