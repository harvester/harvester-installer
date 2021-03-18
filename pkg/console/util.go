package console

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	k3os "github.com/rancher/k3os/pkg/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/rancher/harvester-installer/pkg/config"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	harvesterNodePort  = "30443"
	automaticCmdline   = "harvester.automatic"
)

func getURL(client http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("got %d status code from %s, body: %s", resp.StatusCode, url, string(body))
	}

	return body, nil
}

func validatePingServerURL(url string) error {
	client := http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// After configure the network, network need a few seconds to be available.
	return retryOnError(3, 2, func() error {
		_, err := getURL(client, url)
		return err
	})
}

func retryOnError(retryNum, retryInterval int64, process func() error) error {
	for {
		if err := process(); err != nil {
			if retryNum == 0 {
				return err
			}
			retryNum--
			if retryInterval > 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
			}
			continue
		}
		return nil
	}
}

func getRemoteSSHKeys(url string) ([]string, error) {
	client := http.Client{
		Timeout: defaultHTTPTimeout,
	}
	b, err := getURL(client, url)
	if err != nil {
		return nil, err
	}

	var keys []string
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return nil, errors.Errorf("fail to parse on line %d: %s", i+1, line)
		}
		keys = append(keys, line)
	}
	if len(keys) == 0 {
		return nil, errors.Errorf(("no key found"))
	}
	return keys, nil
}

func getFormattedServerURL(addr string) (string, error) {
	ipErr := checkIP(addr)
	domainErr := checkDomain(addr)
	if ipErr != nil && domainErr != nil {
		return "", fmt.Errorf("%s is not a valid ip/domain", addr)
	}
	return fmt.Sprintf("https://%s:6443", addr), nil
}

func getServerURLFromEnvData(data []byte) (string, error) {
	regexp, err := regexp.Compile("K3S_URL=(.*)\\b")
	if err != nil {
		return "", err
	}
	matches := regexp.FindSubmatch(data)
	if len(matches) == 2 {
		serverURL := string(matches[1])
		i := strings.LastIndex(serverURL, ":")
		if i >= 0 {
			return serverURL[:i] + ":8443", nil
		}
	}
	return "", nil
}

func showNext(c *Console, names ...string) error {
	for _, name := range names {
		v, err := c.GetElement(name)
		if err != nil {
			return err
		}
		if err := v.Show(); err != nil {
			return err
		}
	}

	validatorV, err := c.GetElement(validatorPanel)
	if err != nil {
		return err
	}
	if err := validatorV.Close(); err != nil {
		return err
	}
	return nil
}

func generateHostName() string {
	return "harvester-" + rand.String(5)
}

func getConfigureNetworkCMD(network config.Network) string {
	if network.Method == networkMethodStatic {
		return fmt.Sprintf("/sbin/harvester-configure-network %s %s %s %s %s %s",
			network.Interface,
			network.Method,
			network.IP,
			network.SubnetMask,
			network.Gateway,
			strings.Join(network.DNSNameservers, " "))
	}
	return fmt.Sprintf("/sbin/harvester-configure-network %s %s", network.Interface, networkMethodDHCP)
}

func toCloudConfig(cfg *config.HarvesterConfig) *k3os.CloudConfig {
	cloudConfig := &k3os.CloudConfig{
		K3OS: k3os.K3OS{
			Install: &k3os.Install{},
		},
	}

	// cfg
	cloudConfig.K3OS.ServerURL = cfg.ServerURL
	cloudConfig.K3OS.Token = cfg.Token

	// cfg.OS
	cloudConfig.SSHAuthorizedKeys = dupStrings(cfg.OS.SSHAuthorizedKeys)
	cloudConfig.Hostname = cfg.OS.Hostname
	cloudConfig.K3OS.Modules = dupStrings(cfg.OS.Modules)
	cloudConfig.K3OS.Sysctls = dupStringMap(cfg.OS.Sysctls)
	cloudConfig.K3OS.NTPServers = dupStrings(cfg.OS.NTPServers)
	cloudConfig.K3OS.DNSNameservers = dupStrings(cfg.OS.DNSNameservers)
	if cfg.OS.Wifi != nil {
		cloudConfig.K3OS.Wifi = make([]k3os.Wifi, len(cfg.Wifi))
		for i, w := range cfg.Wifi {
			cloudConfig.K3OS.Wifi[i].Name = w.Name
			cloudConfig.K3OS.Wifi[i].Passphrase = w.Passphrase
		}
	}
	cloudConfig.K3OS.Password = cfg.OS.Password
	cloudConfig.K3OS.Environment = dupStringMap(cfg.OS.Environment)

	// cfg.OS.Install
	cloudConfig.K3OS.Install.ForceEFI = cfg.Install.ForceEFI
	cloudConfig.K3OS.Install.Device = cfg.Install.Device
	cloudConfig.K3OS.Install.Silent = cfg.Install.Silent
	cloudConfig.K3OS.Install.ISOURL = cfg.Install.ISOURL
	cloudConfig.K3OS.Install.PowerOff = cfg.Install.PowerOff
	cloudConfig.K3OS.Install.NoFormat = cfg.Install.NoFormat
	cloudConfig.K3OS.Install.Debug = cfg.Install.Debug
	cloudConfig.K3OS.Install.TTY = cfg.Install.TTY

	for _, network := range cfg.Install.Networks {
		if cloudConfig.Runcmd == nil {
			cloudConfig.Runcmd = []string{}
		}
		if network.Method == networkMethodStatic {
			cloudConfig.Runcmd = append(cloudConfig.Runcmd, getConfigureNetworkCMD(network))
		}
	}

	// k3os & k3s
	cloudConfig.K3OS.Labels = map[string]string{
		"harvester.cattle.io/managed": "true",
	}

	var extraK3sArgs []string
	if cfg.Install.MgmtInterface != "" {
		extraK3sArgs = []string{"--flannel-iface", cfg.Install.MgmtInterface}
	}

	if cfg.Install.Mode == modeJoin {
		cloudConfig.K3OS.K3sArgs = append([]string{"agent"}, extraK3sArgs...)
		return cloudConfig
	}

	cloudConfig.K3OS.K3sArgs = append([]string{
		"server",
		"--cluster-init",
		"--disable",
		"local-storage",
		"--disable",
		"servicelb",
		"--disable",
		"traefik",
		"--cluster-cidr",
		"10.52.0.0/16",
		"--service-cidr",
		"10.53.0.0/16",
		"--cluster-dns",
		"10.53.0.10",
	}, extraK3sArgs...)

	return cloudConfig
}

func doInstall(g *gocui.Gui, cloudConfig *k3os.CloudConfig) error {
	var (
		err      error
		tempFile *os.File
	)

	tempFile, err = ioutil.TempFile("/tmp", "k3os.XXXXXXXX")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	cloudConfig.K3OS.Install.ConfigURL = tempFile.Name()

	ev, err := k3os.ToEnv(*cloudConfig)
	if err != nil {
		return err
	}
	if tempFile != nil {
		cloudConfig.K3OS.Install = nil
		bytes, err := yaml.Marshal(cloudConfig)
		if err != nil {
			return err
		}
		if _, err := tempFile.Write(bytes); err != nil {
			return err
		}
		if err := tempFile.Close(); err != nil {
			return err
		}
		defer os.Remove(tempFile.Name())
	}
	cmd := exec.Command("/usr/libexec/k3os/install")
	cmd.Env = append(os.Environ(), ev...)
	logrus.Infof("env: %v", cmd.Env)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		printToInstallPanel(g, scanner.Text())
	}
	scanner = bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		printToInstallPanel(g, scanner.Text())
	}
	return nil
}

func printToInstallPanel(g *gocui.Gui, message string) {
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(installPanel)
		if err != nil {
			return err
		}
		fmt.Fprintln(v, message)

		lines := len(v.BufferLines())
		_, sy := v.Size()
		if lines > sy {
			ox, oy := v.Origin()
			v.SetOrigin(ox, oy+1)
		}
		return nil
	})
}

func getRemoteConfig(configURL string) (*config.HarvesterConfig, error) {
	client := http.Client{
		Timeout: defaultHTTPTimeout,
	}
	b, err := getURL(client, configURL)
	if err != nil {
		return nil, err
	}
	harvestCfg, err := config.LoadHarvesterConfig(b)
	if err != nil {
		return nil, err
	}
	return harvestCfg, nil
}

func dupStrings(src []string) []string {
	if src == nil {
		return nil
	}
	s := make([]string, len(src))
	copy(s, src)
	return s
}

func dupStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	m := make(map[string]string)
	for k, v := range src {
		m[k] = v
	}
	return m
}
