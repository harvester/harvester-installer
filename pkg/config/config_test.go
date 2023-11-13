package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type SettingManifestMock struct {
	APIVersion string
	Kind       string
	Metadata   map[string]interface{}
	Value      string
}

func (s *SettingManifestMock) assertValueEqual(t *testing.T, expected string) {
	assert.Equal(t, expected, s.Value)
}

func (s *SettingManifestMock) assertNameEqual(t *testing.T, expected string) {
	val := s.Metadata["name"]
	assert.Equal(t, expected, val)
}

func TestHarvesterConfig_sanitized(t *testing.T) {
	c := NewHarvesterConfig()
	c.Password = `#3tQ66t!`
	c.Token = `3mO3&nEJ`
	c.Wifi = []Wifi{{Name: "wifi1", Passphrase: `^s2I8Y2P`}}

	expected := NewHarvesterConfig()
	expected.Password = SanitizeMask
	expected.Token = SanitizeMask
	expected.Wifi = []Wifi{{Name: "wifi1", Passphrase: SanitizeMask}}

	s, err := c.sanitized()
	assert.Equal(t, nil, err)
	assert.Equal(t, expected, s)
}

func TestHarvesterConfig_GetKubeletLabelsArg(t *testing.T) {

	testCases := []struct {
		name      string
		input     map[string]string
		output    []string
		expectErr bool
	}{
		{
			name:   "Successfully creates node-labels argument",
			input:  map[string]string{"labelKey1": "value1"},
			output: []string{"node-labels=labelKey1=value1"},
		},
		{
			name:   "Returns nothing if no Labels is given",
			input:  map[string]string{},
			output: []string{},
		},
		{
			name:      "Error for invalid label name",
			input:     map[string]string{"???invalidName": "value"},
			output:    []string{},
			expectErr: true,
		},
		{
			name:      "Error for invalid label value",
			input:     map[string]string{"example.io/somelabel": "???value###NAH"},
			output:    []string{},
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c := NewHarvesterConfig()
			c.Labels = testCase.input

			result, err := c.GetKubeletArgs()

			if testCase.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t,
					testCase.output,
					result,
				)
			}
		})
	}
}

func TestHarvesterSystemSettingsRendering(t *testing.T) {
	testCases := []struct {
		name         string
		settingName  string
		settingValue string
	}{
		{
			name:         "Test string",
			settingName:  "some-harvester-setting",
			settingValue: "hello, this is setting value",
		},
		{
			name:         "Test boolean encoded as string",
			settingName:  "bool-setting",
			settingValue: "true",
		},
		{
			name:         "Test integer encoded as string",
			settingName:  "int-setting",
			settingValue: "123",
		},
		{
			name:         "Test float encoded as string",
			settingName:  "int-setting",
			settingValue: "123.456",
		},
		{
			name:         "Test JSON encoded value encoded as string",
			settingName:  "json-encoded-setting",
			settingValue: `{"jsonKey": "jsonValue"}`,
		},
	}

	for _, testCase := range testCases {
		// Renders the config into YAML manifest, then decode the YAML manifest and verify the content
		conf := HarvesterConfig{
			SystemSettings: map[string]string{testCase.settingName: testCase.settingValue},
		}
		content, err := render("rancherd-20-harvester-settings.yaml", conf)
		assert.Nil(t, err)

		loadedConf := map[string][]SettingManifestMock{}

		err = yaml.Unmarshal([]byte(content), &loadedConf)
		assert.Nil(t, err)

		// Take the first one
		loadedConf["resources"][0].assertNameEqual(t, testCase.settingName)
		loadedConf["resources"][0].assertValueEqual(t, testCase.settingValue)
	}
}

func TestHarvesterSystemSettingsRendering_MultipleSettings(t *testing.T) {
	// Iterating map is orderless so we need this special test case to test multiple settings
	conf := HarvesterConfig{
		SystemSettings: map[string]string{
			"foo":   "bar",
			"hello": "world",
		},
	}
	content, err := render("rancherd-20-harvester-settings.yaml", conf)
	assert.Nil(t, err)

	loadedConf := map[string][]SettingManifestMock{}
	err = yaml.Unmarshal([]byte(content), &loadedConf)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(loadedConf["resources"]))
	for _, setting := range loadedConf["resources"] {
		switch setting.Value {
		case "bar":
			setting.assertNameEqual(t, "foo")
		case "world":
			setting.assertNameEqual(t, "hello")
		default:
			t.Logf("Unexpected setting value: %s", setting.Value)
			t.Fail()
		}
	}
}

func TestHarvesterSystemSettingsRendering_AsEmptyArrayIfNoSetting(t *testing.T) {
	// If no SystemSettings config, "bootstrapResources" must be rendered as an empty array.
	// If it got rendered as null, it removes every predefined bootstrapResoruces!
	conf := HarvesterConfig{}

	content, err := render("rancherd-20-harvester-settings.yaml", conf)
	assert.Nil(t, err)

	loadedConf := map[string][]SettingManifestMock{}

	err = yaml.Unmarshal([]byte(content), &loadedConf)
	assert.Nil(t, err)

	bootstrapResources, ok := loadedConf["resources"]
	assert.True(t, ok)
	assert.NotNil(t, bootstrapResources)
	assert.Equal(t, 0, len(bootstrapResources))
}

func TestHarvesterTokenRendering(t *testing.T) {
	// Test the Token value is escaped correctly
	testCases := []struct {
		name  string
		token string
	}{
		{
			name:  "Test OWASP password special characters",
			token: " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~",
		},
		{
			name:  "Test mixed characters",
			token: "Hello, I opened a new bar! It's called \"FOOBAR\". \\YES/",
		},
	}

	for _, testCase := range testCases {
		// Renders the config into YAML manifest, then decode the YAML manifest and verify the content
		conf := HarvesterConfig{
			Token:          testCase.token,
			RancherVersion: "v0.0.0-fake", // Necessary to prevent rendering failed
		}
		content, err := render("rancherd-config.yaml", conf)
		assert.Nil(t, err)
		t.Log("Rendered content:")
		t.Log(content)

		loadedConf := map[string]interface{}{}
		t.Log("Loaded Config:")
		t.Log(loadedConf)

		err = yaml.Unmarshal([]byte(content), &loadedConf)
		assert.Nil(t, err)

		assert.Equal(t, loadedConf["token"].(string), testCase.token)
	}
}

func TestHarvesterRootfsRendering(t *testing.T) {
	type Rootfs struct {
		Environment map[string]string
	}

	testCases := []struct {
		name       string
		harvConfig HarvesterConfig
		assertion  func(t *testing.T, rootfs *Rootfs)
	}{
		{
			name:       "Test default config",
			harvConfig: HarvesterConfig{},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.Contains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/harvester/defaultdisk")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.NotContains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/harvester/defaultdisk")
			},
		},
		{
			name: "Test ForceMBR=true and no DataDisk -> No need to mount data partition",
			harvConfig: HarvesterConfig{
				Install: Install{
					ForceMBR: true,
					DataDisk: "",
				},
			},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.NotContains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/harvester/defaultdisk")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/harvester/defaultdisk")
			},
		},
		{
			name: "Test ForceMBR=true but has DataDisk -> Still need to mount data partition",
			harvConfig: HarvesterConfig{
				Install: Install{
					ForceMBR: true,
					DataDisk: "/dev/sdb",
				},
			},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.Contains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/harvester/defaultdisk")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.NotContains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/harvester/defaultdisk")
			},
		},
	}

	for _, tc := range testCases {
		content, err := render("cos-rootfs.yaml", tc.harvConfig)
		assert.NoError(t, err)
		t.Log("Rendered content:")
		t.Log(content)

		rootfs := Rootfs{}
		err = yaml.Unmarshal([]byte(content), &rootfs)
		assert.NoError(t, err)
		t.Log("Loaded Config:")
		t.Log(rootfs)

		tc.assertion(t, &rootfs)
	}
}

func TestNetworkRendering_MTU(t *testing.T) {
	testCases := []struct {
		name         string
		templateName string
		network      Network
		assertion    func(t *testing.T, result string)
	}{
		{
			name:         "MTU = 0 will not set MTU for bond master",
			templateName: "wicked-ifcfg-bond-master",
			network:      Network{MTU: 0},
			assertion: func(t *testing.T, result string) {
				assert.NotContains(t, result, "MTU=")
			},
		},
		{
			name:         "MTU != 0  will set the MTU for bond master",
			templateName: "wicked-ifcfg-bond-master",
			network:      Network{MTU: 1234},
			assertion: func(t *testing.T, result string) {
				assert.Contains(t, result, "MTU=1234")
			},
		},
		{
			name:         "MTU = 0 will not set MTU for eth",
			templateName: "wicked-ifcfg-eth",
			network:      Network{MTU: 0},
			assertion: func(t *testing.T, result string) {
				assert.NotContains(t, result, "MTU=")
			},
		},
		{
			name:         "MTU != 0  will set the MTU for eth",
			templateName: "wicked-ifcfg-eth",
			network:      Network{MTU: 2345},
			assertion: func(t *testing.T, result string) {
				assert.Contains(t, result, "MTU=2345")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := render(tc.templateName, tc.network)
			t.Log(result)
			assert.NoError(t, err)

			tc.assertion(t, result)
		})
	}
}

func TestHarvesterConfigMerge_OtherField(t *testing.T) {
	conf := NewHarvesterConfig()
	conf.Hostname = "hellofoo"
	conf.Labels = map[string]string{"foo": "bar"}
	conf.DNSNameservers = []string{"1.1.1.1"}

	otherConf := NewHarvesterConfig()
	otherConf.Hostname = "NOOOOOOO"
	otherConf.Token = "TokenValue"
	otherConf.Labels = map[string]string{"key": "val"}
	otherConf.DNSNameservers = []string{"8.8.8.8"}

	err := conf.Merge(*otherConf)
	assert.NoError(t, err)

	assert.Equal(t, "hellofoo", conf.Hostname, "Primitive field should not be override")
	assert.Equal(t, map[string]string{"foo": "bar", "key": "val"}, conf.Labels, "Map field should be merged")
	assert.Equal(t, []string{"1.1.1.1", "8.8.8.8"}, conf.DNSNameservers, "Slice shoule be appended")
	assert.Equal(t, "TokenValue", conf.Token, "New field should be added")
}
