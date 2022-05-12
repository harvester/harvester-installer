package console

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
)

// TODO(weihanglo): do not re-implement logic in test.
type FakeValidator struct {
	hasInterfaces []string
	hasDevices    []string
}

func (v FakeValidator) Validate(cfg *config.HarvesterConfig) error {
	if err := v.checkMgmtInterface(cfg.Install.ManagementInterface); err != nil {
		return err
	}
	if err := v.checkDevice(cfg.Install.Device); err != nil {
		return err
	}
	return nil
}

func (v FakeValidator) checkMgmtInterface(network config.Network) error {
	if len(network.Interfaces) > 0 {
		return nil
	}
	return prettyError(ErrMsgMgmtInterfaceNotSpecified, config.MgmtInterfaceName)
}

func (v FakeValidator) checkDevice(device string) error {
	for _, d := range v.hasDevices {
		if d == device {
			return nil
		}
	}
	return prettyError(ErrMsgDeviceNotFound, device)
}

func createDefaultFakeValidator() FakeValidator {
	return FakeValidator{
		hasDevices: []string{"/dev/vda"},
	}
}

func loadConfig(t *testing.T, name string) *config.HarvesterConfig {
	c, err := config.LoadHarvesterConfig(util.LoadFixture(t, name))
	if err != nil {
		t.Fatal("fail to load config: ", err)
	}
	return c
}

func TestValidateConfig(t *testing.T) {
	createCreateConfig := func() *config.HarvesterConfig {
		return &config.HarvesterConfig{
			Token: "token",
			OS: config.OS{
				SSHAuthorizedKeys: []string{"github: someuser"},
				Password:          "password",
			},
			Install: config.Install{
				Mode: config.ModeCreate,
				ManagementInterface: config.Network{
					Interfaces: []config.NetworkInterface{
						{ Name: "eth0" },
					},
				},
				Device: "/dev/vda",
			},
		}
	}

	createJoinConfig := func() *config.HarvesterConfig {
		c := createCreateConfig()
		c.ServerURL = "https://somewhere"
		c.Mode = config.ModeJoin
		return c
	}

	testCases := []struct {
		name     string
		cfg      *config.HarvesterConfig
		preApply func(c *config.HarvesterConfig)
		errMsg   string
	}{
		{
			name: "valid create config",
			cfg:  createCreateConfig(),
		},
		{
			name: "invalid create config: contains server URL",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.ServerURL = "https://somewhere"
			},
			errMsg: ErrMsgModeCreateContainsServerURL,
		},
		{
			name: "invalid config: unknown mode",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.Mode = "asdf"
			},
			errMsg: ErrMsgModeUnknown,
		},
		{
			name: "valid join config",
			cfg:  createJoinConfig(),
		},
		{
			name: "invalid join config: no server URL",
			cfg:  createJoinConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.ServerURL = ""
			},
			errMsg: ErrMsgModeJoinServerURLNotSpecified,
		},
		{
			name: "invalid create config: contains no credential",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.SSHAuthorizedKeys = nil
				c.Password = ""
			},
			errMsg: ErrMsgNoCredentials,
		},
		{
			name: "invalid create config: device not found",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.Device = "/dev/vdb"
			},
			errMsg: ErrMsgDeviceNotFound,
		},
		{
			name: "invalid create config: interface not found",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.Install.ManagementInterface.Interfaces = nil
			},
			errMsg: ErrMsgMgmtInterfaceNotSpecified,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.preApply != nil {
				testCase.preApply(testCase.cfg)
			}
			err := validateConfig(createDefaultFakeValidator(), testCase.cfg)
			if testCase.errMsg == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), testCase.errMsg)
			}
		})
	}
}

func TestCheckToken(t *testing.T) {
	testCases := []struct {
		name       string
		tokenValue string
		expectErr  bool
	}{
		{
			name:       "Some regular string",
			tokenValue: "AlphanumericValue12345",
			expectErr:  false,
		},
		{
			name:       "Some special characters",
			tokenValue: "Hello, @Harvester, you're \"awesome\"! [md](url)",
			expectErr:  false,
		},
		{
			name:       "Non-ASCII characters are invalid",
			tokenValue: "Äöé",
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkToken(tc.tokenValue)
			if tc.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
