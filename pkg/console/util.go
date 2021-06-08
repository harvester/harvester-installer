package console

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
	"golang.org/x/net/http/httpproxy"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/harvester/harvester-installer/pkg/config"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	harvesterNodePort  = "30443"
	automaticCmdline   = "harvester.automatic"
)

func newProxyClient() http.Client {
	return http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			Proxy: proxyFromEnvironment,
		},
	}
}

func proxyFromEnvironment(req *http.Request) (*url.URL, error) {
	return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
}

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
	client := newProxyClient()
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

func toCloudConfig(cfg *config.HarvesterConfig) (*k3os.CloudConfig, error) {
	cloudConfig, err := config.ConvertToK3OS(cfg)
	if err != nil {
		return nil, err
	}

	// remove the /dev/loop directory as the workaround for https://github.com/harvester/harvester/issues/665
	cloudConfig.Runcmd = append(cloudConfig.Runcmd, "rm -rf /dev/loop")

	for _, network := range cfg.Install.Networks {
		if network.Method == networkMethodStatic {
			cloudConfig.Runcmd = append(cloudConfig.Runcmd, getConfigureNetworkCMD(network))
		}
	}

	// k3os & k3s
	cloudConfig.K3OS.Labels = map[string]string{
		"harvesterhci.io/managed": "true",
	}

	var extraK3sArgs []string
	if cfg.Install.MgmtInterface != "" {
		extraK3sArgs = []string{"--flannel-iface", cfg.Install.MgmtInterface}
	}

	if cfg.Install.Mode == modeJoin {
		cloudConfig.K3OS.K3sArgs = append([]string{"agent"}, extraK3sArgs...)
		return cloudConfig, nil
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

	return cloudConfig, nil
}

func execute(g *gocui.Gui, env []string, cmdName string) error {
	cmd := exec.Command(cmdName)
	cmd.Env = env
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
		printToPanel(g, scanner.Text(), installPanel)
	}
	scanner = bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		printToPanel(g, scanner.Text(), installPanel)
	}
	return cmd.Wait()
}

func doInstall(g *gocui.Gui, cloudConfig *k3os.CloudConfig, webhooks RendererWebhooks) error {
	webhooks.Handle(EventInstallStarted)

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

	env := append(os.Environ(), ev...)
	if err := execute(g, env, "/usr/libexec/k3os/install"); err != nil {
		webhooks.Handle(EventInstallFailed)
		return err
	}
	webhooks.Handle(EventInstallSuceeded)
	if err := execute(g, env, "/usr/libexec/k3os/shutdown"); err != nil {
		return err
	}
	return nil
}

func doUpgrade(g *gocui.Gui) error {
	cmd := exec.Command("/k3os/system/k3os/current/harvester-upgrade.sh")
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
		printToPanel(g, scanner.Text(), upgradePanel)
	}
	scanner = bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		printToPanel(g, scanner.Text(), upgradePanel)
	}
	return nil
}

func printToPanel(g *gocui.Gui, message string, panelName string) {
	// block printToPanel call in the same goroutine.
	// This ensures messages are printed out in the calling order.
	ch := make(chan struct{})

	g.Update(func(g *gocui.Gui) error {

		defer func() {
			ch <- struct{}{}
		}()

		v, err := g.View(panelName)
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

	<-ch
}

func getRemoteConfig(configURL string) (*config.HarvesterConfig, error) {
	client := newProxyClient()
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

func retryRemoteConfig(configURL string, g *gocui.Gui) (*config.HarvesterConfig, error) {
	var confData []byte
	client := newProxyClient()

	retries := 30
	interval := 10
	err := retryOnError(int64(retries), int64(interval), func() error {
		var e error
		confData, e = getURL(client, configURL)
		if e != nil {
			logrus.Error(e)
			printToPanel(g, e.Error(), installPanel)
			printToPanel(g, fmt.Sprintf("Retry after %d seconds (Remaining: %d)...", interval, retries), installPanel)
			retries--
		}
		return e
	})

	if err != nil {
		return nil, fmt.Errorf("Fail to fetch config: %w", err)
	}

	harvestCfg, err := config.LoadHarvesterConfig(confData)
	if err != nil {
		return nil, fmt.Errorf("Fail to load config: %w", err)
	}
	return harvestCfg, nil
}

// harvesterInstalled check existing harvester installation by partition label
func harvesterInstalled() (bool, error) {
	output, err := exec.Command("blkid", "-L", "HARVESTER_STATE").CombinedOutput()
	if err != nil {
		return false, err
	}
	if string(output) != "" {
		return true, nil
	}

	return false, nil
}
