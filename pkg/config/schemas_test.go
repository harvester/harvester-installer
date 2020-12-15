package config

import (
	"testing"

	"github.com/rancher/k3os/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestToCloudConfig(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected *config.CloudConfig
		err      error
	}{
		{
			input: []byte(`ssh_authorized_keys:
- ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB...
- github:username
hostname: myhost
init_cmd:
- "echo hello, init command"
boot_cmd:
- "echo hello, boot command"
run_cmd:
- "echo hello, run command"
k3os:
  data_sources:
  - aws
  - cdrom
  modules:
  - kvm
  - nvme
  sysctl:
    kernel.printk: "4 4 1 7"
    kernel.kptr_restrict: "1"
  dns_nameservers:
  - 8.8.8.8
  - 1.1.1.1
  ntp_servers:
  - 0.us.pool.ntp.org
  - 1.us.pool.ntp.org
  wifi:
  - name: home
    passphrase: mypassword
  - name: nothome
    passphrase: somethingelse
  password: rancher
  server_url: https://someserver:6443
  token: TOKEN_VALUE
  labels:
    region: us-west-1
    somekey: somevalue
  k3s_args:
  - server
  - "--disable-agent"
  environment:
    http_proxy: http://myserver
    https_proxy: http://myserver
  taints:
  - key1=value1:NoSchedule
  - key1=value1:NoExecute
`),
			expected: &config.CloudConfig{
				SSHAuthorizedKeys: []string{
					"ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB...",
					"github:username",
				},
				Hostname: "myhost",
				Initcmd:  []string{"echo hello, init command"},
				Runcmd:   []string{"echo hello, run command"},
				Bootcmd:  []string{"echo hello, boot command"},
				K3OS: config.K3OS{
					DataSources: []string{
						"aws",
						"cdrom",
					},
					Modules: []string{
						"kvm",
						"nvme",
					},
					Sysctls: map[string]string{
						"kernel.printk":        "4 4 1 7",
						"kernel.kptr_restrict": "1",
					},
					DNSNameservers: []string{
						"8.8.8.8",
						"1.1.1.1",
					},
					NTPServers: []string{
						"0.us.pool.ntp.org",
						"1.us.pool.ntp.org",
					},
					Wifi: []config.Wifi{
						{
							Name:       "home",
							Passphrase: "mypassword",
						},
						{
							Name:       "nothome",
							Passphrase: "somethingelse",
						},
					},
					Password:  "rancher",
					ServerURL: "https://someserver:6443",
					Token:     "TOKEN_VALUE",
					Labels: map[string]string{
						"region":  "us-west-1",
						"somekey": "somevalue",
					},
					K3sArgs: []string{
						"server",
						"--disable-agent",
					},
					Environment: map[string]string{
						"http_proxy":  "http://myserver",
						"https_proxy": "http://myserver",
					},
					Taints: []string{
						"key1=value1:NoSchedule",
						"key1=value1:NoExecute",
					},
				},
			},
			err: nil,
		},
	}

	for _, testCase := range testCases {
		output, err := ToCloudConfig(testCase.input)
		assert.Equal(t, testCase.expected, output)
		assert.Equal(t, testCase.err, err)
	}
}
