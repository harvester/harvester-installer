package console

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
	yipSchema "github.com/mudler/yip/pkg/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http/httpproxy"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/harvester/harvester-installer/pkg/config"
)

const (
	rancherManagementPort = "8443"
	defaultHTTPTimeout    = 15 * time.Second
	harvesterNodePort     = "30443"
	automaticCmdline      = "harvester.automatic"
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

func validateNTPServers(ntpServerList []string) error {
	for _, ntpServer := range ntpServerList {
		host, port, err := net.SplitHostPort(ntpServer)
		if err != nil {
			if addrErr, ok := err.(*net.AddrError); ok && addrErr.Err == "missing port in address" {
				host = ntpServer
				// default ntp server port
				// RFC: https://datatracker.ietf.org/doc/html/rfc4330#section-4
				port = "123"
			} else {
				return err
			}
		}
		// ntp servers use udp protocol
		// RFC: https://datatracker.ietf.org/doc/html/rfc4330
		conn, err := net.Dial("udp", fmt.Sprintf("%s:%s", host, port))
		if err != nil {
			return err
		}
		if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
			return err
		}

		// RFC: https://datatracker.ietf.org/doc/html/rfc4330#section-4
		// NTP Packet is 48 bytes and we set the first byte for request.
		// 00 100 011 (or 0x2B)
		// |  |   +-- client mode (3)
		// |  + ----- version (4)
		// + -------- leap year indicator, 0 no warning
		req := make([]byte, 48)
		req[0] = 0x2B

		// send time request
		if err := binary.Write(conn, binary.BigEndian, req); err != nil {
			return err
		}

		// block to receive server response
		rsp := make([]byte, 48)
		if err := binary.Read(conn, binary.BigEndian, &rsp); err != nil {
			return err
		}
		conn.Close()
	}

	return nil
}

func enableNTPServers(ntpServerList []string) error {
	if len(ntpServerList) == 0 {
		return nil
	}

	cfg, err := ini.Load("/etc/systemd/timesyncd.conf")
	if err != nil {
		return err
	}

	cfg.Section("Time").Key("NTP").SetValue(strings.Join(ntpServerList, " "))
	cfg.SaveTo("/etc/systemd/timesyncd.conf")

	// When users want to reset NTP servers, we should stop timesyncd first,
	// so it can reload timesyncd.conf after restart.
	output, err := exec.Command("timedatectl", "set-ntp", "false").CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return err
	}

	output, err = exec.Command("timedatectl", "set-ntp", "true").CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return err
	}

	return nil
}

func updateDNSServersAndReloadNetConfig(dnsServerList []string) error {
	dnsServers := strings.Join(dnsServerList, " ")
	output, err := exec.Command("sed", "-i", fmt.Sprintf(`s/^NETCONFIG_DNS_STATIC_SERVERS.*/NETCONFIG_DNS_STATIC_SERVERS="%s"/`, dnsServers), "/etc/sysconfig/network/config").CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return err
	}

	output, err = exec.Command("netconfig", "update", "-m", "dns").CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return err
	}

	return nil
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
	return fmt.Sprintf("https://%s:%s", addr, rancherManagementPort), nil
}

func getServerURLFromRancherdConfig(data []byte) (string, error) {
	rancherdConf := make(map[string]interface{})
	err := yaml.Unmarshal(data, rancherdConf)
	if err != nil {
		return "", err
	}

	if server, ok := rancherdConf["server"]; ok {
		serverURL, typeOK := server.(string)
		if typeOK {
			return serverURL, nil
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

func saveTemp(obj interface{}, prefix string) (string, error) {
	tempFile, err := ioutil.TempFile("/tmp", fmt.Sprintf("%s.", prefix))
	if err != nil {
		return "", err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	if _, err := tempFile.Write(bytes); err != nil {
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func doInstall(g *gocui.Gui, hvstConfig *config.HarvesterConfig, cosConfig *yipSchema.YipConfig, webhooks RendererWebhooks) error {
	webhooks.Handle(EventInstallStarted)

	cosConfigFile, err := saveTemp(cosConfig, "cos")
	if err != nil {
		return err
	}
	defer os.Remove(cosConfigFile)

	hvstConfigFile, err := saveTemp(hvstConfig, "harvester")
	if err != nil {
		return err
	}
	defer os.Remove(hvstConfigFile)

	hvstConfig.Install.ConfigURL = cosConfigFile

	ev, err := hvstConfig.ToCosInstallEnv()
	if err != nil {
		return nil
	}

	env := append(os.Environ(), ev...)
	env = append(env, fmt.Sprintf("HARVESTER_CONFIG=%s", hvstConfigFile))
	if err := execute(g, env, "/usr/sbin/harv-install"); err != nil {
		webhooks.Handle(EventInstallFailed)
		return err
	}
	webhooks.Handle(EventInstallSuceeded)

	if err := execute(g, env, "/usr/sbin/cos-installer-shutdown"); err != nil {
		webhooks.Handle(EventInstallFailed)
		return err
	}

	return nil
}

func doUpgrade(g *gocui.Gui) error {
	// TODO(kiefer): to cOS upgrade method
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
	return false, nil
}
