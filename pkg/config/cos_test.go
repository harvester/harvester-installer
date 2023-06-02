package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/util"
)

func TestCalcCosPersistentPartSize(t *testing.T) {
	testCases := []struct {
		diskSize      uint64
		partitionSize string
		result        uint64
		err           string
	}{
		{
			diskSize:      300,
			partitionSize: "150Gi",
			result:        153600,
		},
		{
			diskSize:      500,
			partitionSize: "153600Mi",
			result:        153600,
		},
		{
			diskSize:      250,
			partitionSize: "240Gi",
			err:           "Partition size is too large. Maximum 176Gi is allowed",
		},
		{
			diskSize:      150,
			partitionSize: "100Gi",
			err:           "Disk size is too small. Minimum 250Gi is required",
		},
		{
			diskSize:      300,
			partitionSize: "153600Ki",
			err:           "Partition size should be ended with 'Mi', 'Gi', and no dot and negative is allowed",
		},
		{
			diskSize:      2000,
			partitionSize: "1.5Ti",
			err:           "Partition size should be ended with 'Mi', 'Gi', and no dot and negative is allowed",
		},
		{
			diskSize:      500,
			partitionSize: "abcd",
			err:           "Partition size should be ended with 'Mi', 'Gi', and no dot and negative is allowed",
		},
	}

	for _, tc := range testCases {
		result, err := calcCosPersistentPartSize(tc.diskSize, tc.partitionSize)
		assert.Equal(t, tc.result, result)
		if err != nil {
			assert.EqualError(t, err, tc.err)
		}
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
