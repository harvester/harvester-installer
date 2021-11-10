package config

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
)

type SettingManifestMock struct {
	APIVersion string
	Kind       string
	Metadata   map[string]interface{}
}

func (s *SettingManifestMock) assertPreconfigEqual(t *testing.T, expected string) {
	annotations := s.Metadata["annotations"].(map[string]interface{})
	val := annotations["harvesterhci.io/preconfigValue"]
	assert.Equal(t, expected, val)
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
		content, err := render(harvesterSystemSettings, conf)
		assert.Nil(t, err)

		manifest := SettingManifestMock{}
		err = yaml.Unmarshal([]byte(content), &manifest)
		assert.Nil(t, err)

		manifest.assertNameEqual(t, testCase.settingName)
		manifest.assertPreconfigEqual(t, testCase.settingValue)
	}
}
