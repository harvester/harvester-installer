package config

import (
	yipSchema "github.com/mudler/yip/pkg/schema"
	"strings"
	"testing"

	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/stretchr/testify/assert"
)

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
			input:       SoftMinDiskSizeGiB,
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

func TestConvertToCos_SSHKeysInYipNetworkStage(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)

	yipConfig, err := ConvertToCOS(conf, false)
	assert.NoError(t, err)

	assert.Equal(t, yipConfig.Stages["network"][0].SSHKeys["rancher"], conf.OS.SSHAuthorizedKeys)
	assert.Nil(t, yipConfig.Stages["initramfs"][0].SSHKeys)
}

func TestConvertToCos_InstallModeOnly(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)

	yipConfig, err := ConvertToCOS(conf, true)
	assert.NoError(t, err)

	assert.NotNil(t, yipConfig.Stages["rootfs"])
	assert.Nil(t, yipConfig.Stages["network"])
	assert.NotNil(t, yipConfig.Stages["initramfs"])
	assert.Equal(t, yipConfig.Stages["initramfs"][0].Users[cosLoginUser], yipSchema.User{
		PasswordHash: conf.OS.Password,
	})
}

func Test_GenerateRancherdConfig(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)

	yipConfig, err := GenerateRancherdConfig(conf)
	assert.NoError(t, err)
	assert.Equal(t, yipConfig.Stages["live"][0].TimeSyncd["NTP"], strings.Join(conf.OS.NTPServers, " "))
	assert.Contains(t, yipConfig.Stages["live"][0].Commands, "wicked ifreload all")
}
