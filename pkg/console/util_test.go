package console

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/util"
)

func TestGetSSHKeysFromURL(t *testing.T) {
	testCases := []struct {
		name         string
		httpResp     string
		pubKeysCount int
		expectError  string
	}{
		{
			name:         "Two public keys",
			httpResp:     string(util.LoadFixture(t, "keys")),
			pubKeysCount: 2,
		},
		{
			name:        "Invalid public key",
			httpResp:    "\nooxx",
			expectError: "fail to parse on line 2: ooxx",
		},
		{
			name:        "No public key",
			httpResp:    "",
			expectError: "no key found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, testCase.httpResp)
			}))
			defer ts.Close()

			pubKeys, err := getRemoteSSHKeys(ts.URL)
			if testCase.expectError != "" {
				assert.EqualError(t, err, testCase.expectError)
			} else {
				assert.Equal(t, nil, err)
				assert.Equal(t, testCase.pubKeysCount, len(pubKeys))
			}
		})
	}
}

func TestGetHStatus(t *testing.T) {
	s := getHarvesterStatus()
	t.Log(s)
}

func TestGetFormattedServerURL(t *testing.T) {
	testCases := []struct {
		Name   string
		input  string
		output string
		err    error
	}{
		{
			Name:   "ip",
			input:  "1.2.3.4",
			output: "https://1.2.3.4:443",
			err:    nil,
		},
		{
			Name:   "domain name",
			input:  "example.org",
			output: "https://example.org:443",
			err:    nil,
		},
		{
			Name:   "invalid ip",
			input:  "1.2.3.4/",
			output: "",
			err:    errors.New("1.2.3.4/ is not a valid ip/domain"),
		},
		{
			Name:   "invalid domain",
			input:  "example.org/",
			output: "",
			err:    errors.New("example.org/ is not a valid ip/domain"),
		},
	}
	for _, testCase := range testCases {
		got, err := getFormattedServerURL(testCase.input)
		assert.Equal(t, testCase.output, got)
		assert.Equal(t, testCase.err, err)
	}
}

func TestF(t *testing.T) {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			if v, ok := addr.(*net.IPNet); ok && !v.IP.IsLoopback() && v.IP.To4() != nil {
				t.Log(v.IP.String())
			}
		}
	}
}

func TestGetServerURLFromRancherdConfig(t *testing.T) {
	testCases := []struct {
		input []byte
		url   string
		err   error
	}{
		{
			input: []byte("role: cluster-init\nkubernetesVersion: v1.21.2+rke2r1"),
			url:   "",
			err:   nil,
		},
		{
			input: []byte("role: agent\nkubernetesVersion: v1.21.2+rke2r1\nserver: https://172.0.0.1:443"),
			url:   "https://172.0.0.1:443",
			err:   nil,
		},
	}

	for _, testCase := range testCases {
		url, err := getServerURLFromRancherdConfig(testCase.input)
		assert.Equal(t, testCase.url, url)
		assert.Equal(t, testCase.err, err)
	}
}

func TestValidateNTPServers(t *testing.T) {
	quit := make(chan interface{})
	mockNTPServers, err := startMockNTPServers(quit)
	if err != nil {
		t.Fatalf("can't start mock ntp servers, %v", err)
	}
	testCases := []struct {
		name        string
		input       []string
		expectError bool
	}{
		{
			name:        "Correct NTP Servers",
			input:       mockNTPServers,
			expectError: false,
		},
		{
			name:        "Empty input",
			input:       []string{},
			expectError: false,
		},
		{
			name:        "Invalid URL",
			input:       []string{"error"},
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateNTPServers(testCase.input)
			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				if err != nil {
					t.Log(err)
				}
				assert.Nil(t, err)
			}
		})
	}
	close(quit)
}

func startMockNTPServers(quit chan interface{}) ([]string, error) {
	ntpServers := []string{}
	for i := 0; i < 2; i++ {
		listener, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		ntpServers = append(ntpServers, listener.LocalAddr().String())

		go func(listener net.PacketConn) {
			defer listener.Close()

			for {
				req := make([]byte, 48)
				_, addr, err := listener.ReadFrom(req)
				if err != nil {
					select {
					case <-quit:
						return
					default:
						continue
					}
				}
				go func(listener net.PacketConn, addr net.Addr) {
					listener.WriteTo(make([]byte, 48), addr)
				}(listener, addr)
			}

		}(listener)
	}
	return ntpServers, nil
}

func TestCalcCosPersistentPartSize(t *testing.T) {
	testCases := []struct {
		name        string
		input       uint64
		output      uint64
		expectError bool
	}{
		{
			name:        "Disk too small",
			input:       50,
			output:      0,
			expectError: true,
		},
		{
			name:        "Disk meet hard requirement",
			input:       60,
			output:      25,
			expectError: false,
		},
		{
			name:        "Disk a bit larger than hard requirement: 80G",
			input:       80,
			output:      31,
			expectError: false,
		},
		{
			name:        "Disk a bit larger than hard requirement: 100G",
			input:       100,
			output:      37,
			expectError: false,
		},
		{
			name:        "Disk close to the soft requirement",
			input:       139,
			output:      49,
			expectError: false,
		},
		{
			name:        "Disk meet soft requirement",
			input:       softMinDiskSizeGiB,
			output:      50,
			expectError: false,
		},
		{
			name:        "200GiB",
			input:       200,
			output:      60,
			expectError: false,
		},
		{
			name:        "300GiB",
			input:       300,
			output:      70,
			expectError: false,
		},
		{
			name:        "400GiB",
			input:       400,
			output:      80,
			expectError: false,
		},
		{
			name:        "500GiB",
			input:       500,
			output:      90,
			expectError: false,
		},
		{
			name:        "600GiB",
			input:       600,
			output:      100,
			expectError: false,
		},
		{
			name:        "Greater than 600GiB should still get 100",
			input:       700,
			output:      100,
			expectError: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sizeGiB, err := calcCosPersistentPartSize(testCase.input)
			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				if err != nil {
					t.Log(err)
				}
				assert.Equal(t, sizeGiB, testCase.output)
			}
		})
	}
}
