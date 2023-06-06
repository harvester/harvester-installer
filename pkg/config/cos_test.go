package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/util"
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
			input:       200,
			output:      25,
			expectError: false,
		},
		{
			name:        "Disk a bit larger than hard requirement: 250G",
			input:       250,
			output:      29,
			expectError: false,
		},
		{
			name:        "Disk a bit larger than hard requirement: 280G",
			input:       280,
			output:      31,
			expectError: false,
		},
		{
			name:        "Disk close to the soft requirement",
			input:       499,
			output:      49,
			expectError: false,
		},
		{
			name:        "Disk meet soft requirement",
			input:       SoftMinDiskSizeGiB,
			output:      90,
			expectError: false,
		},
		{
			name:        "200GiB",
			input:       200,
			output:      25,
			expectError: false,
		},
		{
			name:        "500GiB",
			input:       300,
			output:      33,
			expectError: false,
		},
		{
			name:        "400GiB",
			input:       400,
			output:      41,
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
				if sizeGiB != testCase.output {
					out := fmt.Sprintf("Argument: %v\n", testCase.name) +
						fmt.Sprintf("Expected result: %v\n", testCase.output) +
						fmt.Sprintf("Actual result: %v\n", sizeGiB)
					t.Fatalf(out)
				}
			}
		})
	}
}

func TestConvertToCos_SSHKeysInYipNetworkStage(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)

	yipConfig, err := ConvertToCOS(conf)
	assert.NoError(t, err)

	assert.Equal(t, yipConfig.Stages["network"][0].SSHKeys["rancher"], conf.OS.SSHAuthorizedKeys)
	assert.Nil(t, yipConfig.Stages["initramfs"][0].SSHKeys)
}
