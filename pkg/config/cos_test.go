package config

import (
	"strings"
	"testing"

	yipSchema "github.com/mudler/yip/pkg/schema"
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
			err:           "Installation disk size is too small. Minimum 250Gi is required",
		},
		{
			diskSize:      300,
			partitionSize: "153600Ki",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      2000,
			partitionSize: "1.5Ti",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      500,
			partitionSize: "abcd",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
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

func TestConvertToCos_InstallModeOnly(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)
	conf.Mode = ModeInstall
	yipConfig, err := ConvertToCOS(conf)
	assert.NoError(t, err)

	assert.NotNil(t, yipConfig.Stages["rootfs"])
	assert.Len(t, yipConfig.Stages["network"][0].SSHKeys, 0)
	assert.NotNil(t, yipConfig.Stages["initramfs"])
	assert.Equal(t, yipConfig.Stages["initramfs"][0].Users[cosLoginUser], yipSchema.User{
		PasswordHash: conf.OS.Password,
	})
}

func Test_GenerateRancherdConfig(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)
	conf.Mode = ModeInstall
	yipConfig, err := GenerateRancherdConfig(conf)
	assert.NoError(t, err)
	assert.Equal(t, yipConfig.Stages["live"][0].TimeSyncd["NTP"], strings.Join(conf.OS.NTPServers, " "))
	assert.Contains(t, yipConfig.Stages["live"][0].Commands, "wicked ifreload all")
}

func TestConvertToCos_VerifyNetworkCreateMode(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)
	yipConfig, err := ConvertToCOS(conf)
	assert.NoError(t, err)
	assert.Contains(t, yipConfig.Stages["initramfs"][0].Commands, "sed -i 's/^NETCONFIG_DNS_STATIC_SERVERS.*/NETCONFIG_DNS_STATIC_SERVERS=\"8.8.8.8 1.1.1.1\"/' /etc/sysconfig/network/config")
	assert.Contains(t, yipConfig.Stages["initramfs"][0].Commands, "rm -f /etc/sysconfig/network/ifroute-mgmt-br")
	assert.True(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/rancher/rancherd/config.yaml"))
	assert.True(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-mgmt-bo"))
	assert.True(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-mgmt-br"))
	assert.True(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-ens0"))
	assert.True(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-ens3"))

}

func TestConvertToCos_VerifyNetworkInstallMode(t *testing.T) {
	conf, err := LoadHarvesterConfig(util.LoadFixture(t, "harvester-config.yaml"))
	assert.NoError(t, err)
	conf.Mode = ModeInstall
	yipConfig, err := ConvertToCOS(conf)
	assert.NoError(t, err)
	assert.NotContains(t, yipConfig.Stages["initramfs"][0].Commands, "sed -i 's/^NETCONFIG_DNS_STATIC_SERVERS.*/NETCONFIG_DNS_STATIC_SERVERS=\"8.8.8.8 1.1.1.1\"/' /etc/sysconfig/network/config")
	assert.NotContains(t, yipConfig.Stages["initramfs"][0].Commands, "rm -f /etc/sysconfig/network/ifroute-harvester-mgmt")
	assert.False(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-ens0"))
	assert.False(t, containsFile(yipConfig.Stages["initramfs"][0].Files, "/etc/sysconfig/network/ifcfg-ens3"))
}

func containsFile(files []yipSchema.File, fileName string) bool {
	for _, v := range files {
		if v.Path == fileName {
			return true
		}
	}
	return false
}
