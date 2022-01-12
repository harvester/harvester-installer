package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
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
		loadedConf["bootstrapResources"][0].assertNameEqual(t, testCase.settingName)
		loadedConf["bootstrapResources"][0].assertValueEqual(t, testCase.settingValue)
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

	assert.Equal(t, 2, len(loadedConf["bootstrapResources"]))
	for _, setting := range loadedConf["bootstrapResources"] {
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

	bootstrapResources, ok := loadedConf["bootstrapResources"]
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
			Token: testCase.token,
			// RuntimeVersion: "DoesNotMatter",
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
				assert.Contains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/longhorn_data")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.NotContains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn_data")
			},
		},
		{
			name: "Test NoDataPartition true",
			harvConfig: HarvesterConfig{
				Install: Install{
					NoDataPartition: true,
				},
			},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.NotContains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/longhorn_data")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn_data")
			},
		},
		{
			name: "Test DataDisk configured",
			harvConfig: HarvesterConfig{
				Install: Install{
					DataDisk: "/dev/sda",
				},
			},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.Contains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/longhorn_data")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn")
				assert.NotContains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/longhorn_data")
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
