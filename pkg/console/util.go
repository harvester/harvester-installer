package console

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http/httpproxy"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/harvester/harvester-installer/pkg/widgets"
)

const (
	rancherManagementPort = "443"
	defaultHTTPTimeout    = 15 * time.Second
	automaticCmdline      = "harvester.automatic"
	installFailureMessage = `
** Installation Failed **
You can see the full installation log by:
  - Press CTRL + ALT + F2 to switch to a different TTY console.
  - Login with user "rancher" (password is "rancher").
  - Run the command: less %s.
`
	https = "https://"

	ElementalConfigDir  = "/tmp/elemental"
	ElementalConfigFile = "config.yaml"
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
		var err error
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

		ips, err := net.LookupIP(host)
		if err != nil {
			return err
		}

		isSuccess := false
		ipStrings := make([]string, 0, len(ips))
		for _, ip := range ips {
			ipString := ip.String()
			ipStrings = append(ipStrings, ipString)
			logrus.Infof("try to validate NTP server %s", ipString)
			// ntp servers use udp protocol
			// RFC: https://datatracker.ietf.org/doc/html/rfc4330
			var conn net.Conn
			address := net.JoinHostPort(ipString, port)
			conn, err = net.Dial("udp", address)
			if err != nil {
				logrus.Errorf("fail to dial %s, err: %v", address, err)
				continue
			}
			defer conn.Close()
			if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				logrus.Errorf("fail to set deadline for connection")
			}

			// RFC: https://datatracker.ietf.org/doc/html/rfc4330#section-4
			// NTP Packet is 48 bytes and we set the first byte for request.
			// 00 100 011 (or 0x23)
			// |  |   +-- client mode (3)
			// |  + ----- version (4)
			// + -------- leap year indicator, 0 no warning
			req := make([]byte, 48)
			req[0] = 0x23

			// send time request
			if err = binary.Write(conn, binary.BigEndian, req); err != nil {
				logrus.Errorf("fail to send NTP request")
				continue
			}

			// block to receive server response
			rsp := make([]byte, 48)
			if err = binary.Read(conn, binary.BigEndian, &rsp); err != nil {
				logrus.Errorf("fail to receive NTP response")
				continue
			}
			isSuccess = true
			break
		}

		if !isSuccess {
			logrus.Errorf("fail to validate NTP servers %v", ipStrings)
			return fmt.Errorf("fail to validate NTP servers: %v, err: %w", ipStrings, err)
		}
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

func diskExceedsMBRLimit(blockDevPath string) (bool, error) {
	// Test if the storage is larger than MBR limit (2TiB).
	// MBR partition table uses 32-bit values to describe the starting offset and length of a
	// partition. Due to this size limit, MBR allows a maximum disk size of
	// (2^32 - 1) = 4,294,967,295 sectors, which is 2,199,023,255,040 bytes (512 bytes per sector)
	output, err := exec.Command("/bin/sh", "-c", fmt.Sprintf(`lsblk %s -n -b -d -r -o SIZE`, blockDevPath)).CombinedOutput()
	if err != nil {
		return false, err
	}
	sizeStr := strings.TrimSpace(string(output))
	sizeByte, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return false, err
	}

	if sizeByte > 2199023255040 {
		return true, nil
	}
	return false, nil
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

	lines := strings.Split(string(b), "\n")
	keys := make([]string, 0, len(lines))
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
	if addr == "" {
		return "", errors.New("management address cannot be empty")
	}
	addr = strings.TrimSpace(addr)

	realAddr := addr
	if !strings.HasPrefix(addr, https) {
		realAddr = https + addr
	}
	parsedURL, err := url.ParseRequestURI(realAddr)
	if err != nil {
		return "", fmt.Errorf("%s is invalid", addr)
	}

	host := parsedURL.Hostname()
	if checkIP(host) != nil && checkDomain(host) != nil {
		return "", fmt.Errorf("%s is not a valid ip/domain", addr)
	}

	if parsedURL.Path != "" {
		return "", fmt.Errorf("path is not allowed in management address: %s", parsedURL.Path)
	}

	port := parsedURL.Port()
	if port == "" {
		parsedURL.Host += ":443"
	} else if port != "443" {
		return "", fmt.Errorf("currently non-443 port are not allowed")
	}

	return parsedURL.String(), nil
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

func execute(ctx context.Context, g *gocui.Gui, env []string, cmdName string) error {
	cmd := exec.CommandContext(ctx, cmdName)
	cmd.Env = env
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	var wg sync.WaitGroup
	var writeLock sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		printToPanelAndLog(g, installPanel, "[stderr]", stderr, &writeLock)
	}()

	go func() {
		defer wg.Done()
		printToPanelAndLog(g, installPanel, "[stdout]", stdout, &writeLock)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()
	return cmd.Wait()
}

func printToPanelAndLog(g *gocui.Gui, panel string, logPrefix string, reader io.Reader, lock *sync.Mutex) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logrus.Infof("%s: %s", logPrefix, scanner.Text())
		lock.Lock()
		printToPanel(g, scanner.Text(), panel)
		lock.Unlock()
	}
}

func saveElementalConfig(obj interface{}) (string, string, error) {
	err := os.MkdirAll(ElementalConfigDir, os.ModePerm)
	if err != nil {
		return "", "", err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return "", "", err
	}

	elementalConfigFile := filepath.Join(ElementalConfigDir, ElementalConfigFile)
	err = ioutil.WriteFile(elementalConfigFile, bytes, os.ModePerm)
	if err != nil {
		return "", "", err
	}

	return ElementalConfigDir, elementalConfigFile, nil
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

	logrus.Infof("Content of %s: %s", tempFile.Name(), string(bytes))

	return tempFile.Name(), nil
}

func doInstall(g *gocui.Gui, hvstConfig *config.HarvesterConfig, webhooks RendererWebhooks) error {
	ctx := context.TODO()
	webhooks.Handle(EventInstallStarted)

	cosConfig, err := config.ConvertToCOS(hvstConfig)
	if err != nil {
		printToPanel(g, err.Error(), installPanel)
		return err
	}
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
	elementalConfig, err := config.ConvertToElementalConfig(hvstConfig)
	if err != nil {
		return err
	}

	// provide HARVESTER_ISO_URL, DEBUG, SILENT
	ev, err := hvstConfig.ToCosInstallEnv()
	if err != nil {
		return nil
	}
	env := append(os.Environ(), ev...)
	env = append(env, fmt.Sprintf("HARVESTER_CONFIG=%s", hvstConfigFile))
	env = append(env, fmt.Sprintf("HARVESTER_INSTALLATION_LOG=%s", defaultLogFilePath))

	if hvstConfig.ShouldCreateDataPartitionOnOsDisk() {
		// Use custom layout (which also creates Longhorn partition) when needed
		elementalConfig, err = config.CreateRootPartitioningLayout(elementalConfig, hvstConfig.Install.Device)
		if err != nil {
			return err
		}
	}

	if hvstConfig.DataDisk != "" {
		env = append(env, fmt.Sprintf("HARVESTER_DATA_DISK=%s", hvstConfig.DataDisk))
	}

	elementalConfigDir, elementalConfigFile, err := saveElementalConfig(elementalConfig)
	if err != nil {
		return nil
	}
	env = append(env, fmt.Sprintf("ELEMENTAL_CONFIG=%s", elementalConfigFile))
	env = append(env, fmt.Sprintf("ELEMENTAL_CONFIG_DIR=%s", elementalConfigDir))

	if err := execute(ctx, g, env, "/usr/sbin/harv-install"); err != nil {
		webhooks.Handle(EventInstallFailed)
		printToPanel(g, fmt.Sprintf(installFailureMessage, defaultLogFilePath), installPanel)
		if hvstConfig.Debug {
			printToPanel(g, "support config is being generated as running in debug mode, this can take a few minutes...", installPanel)
			fileSuffix := fmt.Sprintf("harvester_%s", rand.String(5))
			scErr := executeSupportconfig(ctx, fileSuffix)
			if scErr != nil {
				printToPanel(g, fmt.Sprintf("support config collection failed %v", err), installPanel)
			}
			printToPanel(g, fmt.Sprintf("support config is available at /var/log/scc_%s.txz", fileSuffix), installPanel)
		}
		return err
	}
	webhooks.Handle(EventInstallSuceeded)

	// Enable CTRL-C to stop system from rebooting after installation
	cancellableCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			logrus.Info("Auto-reboot cancelled")
			cancel()
			return quit(g, v)
		}); err != nil {

		return err
	}

	if err := execute(cancellableCtx, g, env, "/usr/sbin/cos-installer-shutdown"); err != nil {
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

func validateDiskSize(devPath string) error {
	diskSizeBytes, err := util.GetDiskSizeBytes(devPath)
	if err != nil {
		return err
	}
	if diskSizeBytes>>30 < config.HardMinDiskSizeGiB {
		return fmt.Errorf("Disk size too small. Minimum %dGB is required", config.HardMinDiskSizeGiB)
	}

	return nil
}

func validateDiskSizeSoft(devPath string) error {
	diskSizeBytes, err := util.GetDiskSizeBytes(devPath)
	if err != nil {
		return err
	}
	if diskSizeBytes>>30 < config.SoftMinDiskSizeGiB {
		return fmt.Errorf("Disk size is smaller than the recommended size: %dGB", config.SoftMinDiskSizeGiB)
	}

	return nil
}

func systemIsBIOS() bool {
	if _, err := os.Stat("/sys/firmware/efi"); os.IsNotExist(err) {
		return true
	}
	return false
}

func canChooseDataDisk() (bool, error) {
	// TODO This is a copy of getDiskOptions(). Deduplicate these two
	output, err := exec.Command("/bin/sh", "-c", `lsblk -r -o NAME,SIZE,TYPE | grep -w disk|cut -d ' ' -f 1,2`).CombinedOutput()
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	var options []widgets.Option
	for _, line := range lines {
		splits := strings.SplitN(line, " ", 2)
		if len(splits) == 2 {
			options = append(options, widgets.Option{
				Value: "/dev/" + splits[0],
				Text:  line,
			})
		}
	}

	return len(options) > 1, nil
}

func createVerticalLocator(c *Console) func(elem widgets.Element, height int) {
	maxX, maxY := c.Gui.Size()
	lastY := maxY / 8
	return func(elem widgets.Element, height int) {
		var (
			x0 = maxX / 8
			y0 = lastY
			x1 = maxX / 8 * 7
			y1 int
		)
		if height == 0 {
			y1 = maxY / 8 * 7
		} else {
			y1 = y0 + height
		}
		lastY += height
		elem.SetLocation(x0, y0, x1, y1)
	}
}

func createVerticalLocatorWithName(c *Console) func(elemName string, height int) error {
	maxX, maxY := c.Gui.Size()
	lastY := maxY / 8
	return func(elemName string, height int) error {
		elem, err := c.GetElement(elemName)
		if err != nil {
			return err
		}

		var (
			x0 = maxX / 8
			y0 = lastY
			x1 = maxX / 8 * 7
			y1 int
		)
		if height == 0 {
			y1 = maxY / 8 * 7
		} else {
			y1 = y0 + height
		}
		lastY += height
		elem.SetLocation(x0, y0, x1, y1)
		return nil
	}
}

func needToGetVIPFromDHCP(mode, vip, hwAddr string) bool {
	return strings.ToLower(mode) == config.NetworkMethodDHCP && (vip == "" || hwAddr == "")
}

func executeSupportconfig(ctx context.Context, fileName string) error {
	cmd := exec.CommandContext(ctx, "/sbin/supportconfig", "-Q", "-B", fileName)

	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
