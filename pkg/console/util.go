package console

import (
	"bufio"
	"bytes"
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
	"github.com/imdario/mergo"
	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	"github.com/rancher/k3os/pkg/config"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/rand"

	cfg "github.com/rancher/harvester-installer/pkg/config"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	harvesterNodePort  = "30443"
	harvesterReplicas  = "3"
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

func validateInsecureURL(url string) error {
	client := http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	_, err := getURL(client, url)
	return err
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

func getFormattedServerURL(addr string) string {
	if !strings.HasPrefix(addr, "https://") {
		addr = "https://" + addr
	}
	if !strings.HasSuffix(addr, ":6443") {
		addr = addr + ":6443"
	}
	return addr
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

func customizeConfig() {
	//common configs for both server and agent
	cfg.Config.K3OS.DNSNameservers = []string{"8.8.8.8"}
	cfg.Config.K3OS.NTPServers = []string{"ntp.ubuntu.com"}
	cfg.Config.K3OS.Modules = []string{"kvm", "vhost_net"}
	cfg.Config.Hostname = "harvester-" + rand.String(5)

	cfg.Config.K3OS.Labels = map[string]string{
		"harvester.cattle.io/managed": "true",
	}

	if cfg.Config.InstallMode == modeJoin {
		cfg.Config.K3OS.K3sArgs = append([]string{"agent"}, cfg.Config.ExtraK3sArgs...)
		return
	}

	var harvesterChartValues = map[string]string{
		"multus.enabled":                                "true",
		"longhorn.enabled":                              "true",
		"minio.persistence.storageClass":                "longhorn",
		"harvester-network-controller.image.pullPolicy": "IfNotPresent",
		"containers.apiserver.image.imagePullPolicy":    "IfNotPresent",
		"containers.apiserver.authMode":                 "localUser",
		"service.harvester.type":                        "NodePort",
		"service.harvester.httpsNodePort":               harvesterNodePort,
		"replicas":                                      harvesterReplicas,
	}

	cfg.Config.WriteFiles = []config.File{
		{
			Owner:              "root",
			Path:               "/var/lib/rancher/k3s/server/manifests/harvester.yaml",
			RawFilePermissions: "0600",
			Content:            getHarvesterManifestContent(harvesterChartValues),
		},
	}
	cfg.Config.K3OS.Labels["svccontroller.k3s.cattle.io/enablelb"] = "true"
	cfg.Config.K3OS.K3sArgs = append([]string{
		"server",
		"--cluster-init",
		"--disable",
		"local-storage",
	}, cfg.Config.ExtraK3sArgs...)
}

func doInstall(g *gocui.Gui) error {
	var (
		err      error
		tempFile *os.File
	)

	if cfg.Config.K3OS.Install.ConfigURL != "" {
		remoteConfig, err := getRemoteCloudConfig(cfg.Config.K3OS.Install.ConfigURL)
		if err != nil {
			printToInstallPanel(g, err.Error())
		} else if err := mergo.Merge(&cfg.Config.CloudConfig, remoteConfig, mergo.WithAppendSlice); err != nil {
			printToInstallPanel(g, err.Error())
		}
	}

	tempFile, err = ioutil.TempFile("/tmp", "k3os.XXXXXXXX")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	cfg.Config.K3OS.Install.ConfigURL = tempFile.Name()

	ev, err := config.ToEnv(cfg.Config.CloudConfig)
	if err != nil {
		return err
	}
	if tempFile != nil {
		cfg.Config.K3OS.Install = nil
		bytes, err := yaml.Marshal(&cfg.Config.CloudConfig)
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

func getRemoteCloudConfig(configURL string) (*config.CloudConfig, error) {
	client := http.Client{
		Timeout: defaultHTTPTimeout,
	}
	b, err := getURL(client, configURL)
	if err != nil {
		return nil, err
	}
	return cfg.ToCloudConfig(b)
}

func getHarvesterManifestContent(values map[string]string) string {
	base := `apiVersion: v1
kind: Namespace
metadata:
  name: harvester-system
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: harvester
  namespace: kube-system
spec:
  chart: https://%{KUBERNETES_API}%/static/charts/harvester-0.1.0.tgz
  targetNamespace: harvester-system
  set:
`
	var buffer = bytes.Buffer{}
	buffer.WriteString(base)
	for k, v := range values {
		buffer.WriteString(fmt.Sprintf("    %s: %q\n", k, v))
	}
	return buffer.String()
}
