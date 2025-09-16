package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/harvester/harvester-installer/pkg/util"
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
			output: []string{"max-pods=200", "node-labels=labelKey1=value1"},
		},
		{
			name:   "Returns maxPods even if no Labels are given",
			input:  map[string]string{},
			output: []string{"max-pods=200"},
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
		{
			name:   "Successfully creates max-pods argument",
			input:  map[string]string{},
			output: []string{"max-pods=200"},
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
		{
			name: "Test additional persistent state paths",
			harvConfig: HarvesterConfig{
				OS: OS{
					PersistentStatePaths: []string{
						"/path1",
						"/path2",
					},
				},
			},
			assertion: func(t *testing.T, rootfs *Rootfs) {
				assert.Contains(t, rootfs.Environment["VOLUMES"], "LABEL=HARV_LH_DEFAULT:/var/lib/harvester/defaultdisk")
				assert.NotContains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/var/lib/harvester/defaultdisk")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/path1")
				assert.Contains(t, rootfs.Environment["PERSISTENT_STATE_PATHS"], "/path2")
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

func TestHarvesterAfterInstallChrootRendering(t *testing.T) {
	type HarvesterAfterInstallChroot struct {
		Commands []string `yaml:"commands,omitempty"`
	}

	testCases := []struct {
		name       string
		harvConfig HarvesterConfig
		assertion  func(t *testing.T, afterInstallChroot *HarvesterAfterInstallChroot)
	}{

		{
			name: "Test after-install-chroot-command",
			harvConfig: HarvesterConfig{
				OS: OS{
					AfterInstallChrootCommands: []string{
						`echo "hello"`,
						`echo "world"`,
					},
				},
			},
			assertion: func(t *testing.T, afterInstallChroot *HarvesterAfterInstallChroot) {
				assert.Contains(t, afterInstallChroot.Commands, `echo "hello"`)
				assert.Contains(t, afterInstallChroot.Commands, `echo "world"`)
			},
		},
	}

	for _, tc := range testCases {
		content, err := render("cos-after-install-chroot.yaml", tc.harvConfig)
		assert.NoError(t, err)
		t.Log("Rendered content:")
		t.Log(content)

		afterInstallChroot := HarvesterAfterInstallChroot{}
		err = yaml.Unmarshal([]byte(content), &afterInstallChroot)
		assert.NoError(t, err)
		t.Log("Loaded Config:")
		t.Log(afterInstallChroot)

		tc.assertion(t, &afterInstallChroot)
	}
}

func TestHarvesterConfigMerge_Addons(t *testing.T) {
	conf := NewHarvesterConfig()
	conf.Hostname = "hellofoo"

	otherConf := NewHarvesterConfig()
	otherConf.Addons = map[string]Addon{
		"rancher-logging":    {true, "the value to overwrite original"},
		"rancher-monitoring": {false, ""},
	}

	err := conf.Merge(*otherConf)
	assert.NoError(t, err)

	assert.Equal(t, true, conf.Addons["rancher-logging"].Enabled, "Addons Enabled true should be merged")
	assert.Equal(t, "the value to overwrite original", conf.Addons["rancher-logging"].ValuesContent, "Addons ValuesContent should be merged")
	assert.Equal(t, false, conf.Addons["rancher-monitoring"].Enabled, "Addons Enabled false should be merged")
}

func TestHarvesterReservedResourcesConfigRendering(t *testing.T) {
	conf := &HarvesterConfig{}
	content, err := render("rke2-99-z00-harvester-reserved-resources.yaml", conf)
	assert.NoError(t, err)

	loadedConf := map[string][]string{}

	err = yaml.Unmarshal([]byte(content), &loadedConf)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(loadedConf["kubelet-arg+"]))

	systemReserved := loadedConf["kubelet-arg+"][0]
	assert.True(t, strings.HasPrefix(systemReserved, "system-reserved=cpu="),
		fmt.Sprintf("%s doesn't started with system-reserved=cpu=", systemReserved))
	systemReservedArray := strings.Split(systemReserved, "system-reserved=cpu=")
	assert.Equal(t, 2, len(systemReservedArray))
	systemCPUReserved, err := strconv.Atoi(strings.Replace(systemReservedArray[1], "m", "", 1))
	assert.NoError(t, err)

	kubeReserved := loadedConf["kubelet-arg+"][1]
	assert.True(t, strings.HasPrefix(kubeReserved, "kube-reserved=cpu="),
		fmt.Sprintf("%s doesn't started with kube-reserved=cpu=", kubeReserved))
	kubeReservedArray := strings.Split(kubeReserved, "kube-reserved=cpu=")
	assert.Equal(t, 2, len(kubeReservedArray))
	kubeCPUReserved, err := strconv.Atoi(strings.Replace(kubeReservedArray[1], "m", "", 1))
	assert.NoError(t, err)

	assert.Equal(t, systemCPUReserved, kubeCPUReserved*2/3)
}

func TestHarvesterAddonsFileRendering(t *testing.T) {
	fname := "rancherd-22-addons.yaml"
	conf := &HarvesterConfig{}
	content, err := render(fname, conf)
	assert.NoError(t, err)
	// the file should not contain any addon templated key, otherwise the file is not fully templated
	assert.False(t, strings.Contains(content, "<<"))
}

func TestHarvesterConstructedAddonsFileRendering(t *testing.T) {
	addonKey := "<<"

	testCases := []struct {
		name            string
		harvConfig      *HarvesterConfig
		fname           string
		rendError       bool
		includeAddonKey bool
	}{
		{
			name:       "error when addon file is malformed",
			harvConfig: &HarvesterConfig{},
			fname:      "./testdata/rancherd-22-fake-addons.yaml",
			rendError:  true,
		},
		{
			name:            "error when addon file is not templated",
			harvConfig:      &HarvesterConfig{},
			fname:           "./testdata/rancherd-22-not-templated-addons.yaml",
			rendError:       false,
			includeAddonKey: true,
		},
		{
			name:            "ok when addon file is well templated",
			harvConfig:      &HarvesterConfig{},
			fname:           "./testdata/rancherd-22-good-addons.yaml",
			rendError:       false,
			includeAddonKey: false,
		},
	}

	for _, tc := range testCases {
		fc, err := os.ReadFile(tc.fname)
		assert.NoError(t, err)

		conf := &HarvesterConfig{}
		// render() can only work with files under templates, this test runs with files under testdata path
		content, err := util.RenderTemplate(string(fc), conf)
		if tc.rendError {
			assert.Error(t, err)
			continue
		}
		if tc.includeAddonKey {
			assert.True(t, strings.Contains(content, addonKey))
		}
	}
}

func TestCalculateCPUReservedInMilliCPU(t *testing.T) {
	testCases := []struct {
		name               string
		coreNum            int
		maxPods            int
		reservedMilliCores int64
	}{
		{
			name:               "invalid core num",
			coreNum:            -1,
			maxPods:            MaxPods,
			reservedMilliCores: 0,
		},
		{
			name:               "invalid max pods",
			coreNum:            1,
			maxPods:            -1,
			reservedMilliCores: 0,
		},
		{
			name:               "core = 1 and max pods = 110",
			coreNum:            1,
			maxPods:            110,
			reservedMilliCores: 60,
		},
		{
			name:               "core = 1",
			coreNum:            1,
			maxPods:            MaxPods,
			reservedMilliCores: 60 + 400,
		},
		{
			name:               "core = 2",
			coreNum:            2,
			maxPods:            MaxPods,
			reservedMilliCores: 60 + 10 + 400,
		},
		{
			name:               "core = 4",
			coreNum:            4,
			maxPods:            MaxPods,
			reservedMilliCores: 60 + 10 + 5*2 + 400,
		},
		{
			name:               "core = 8",
			coreNum:            8,
			maxPods:            MaxPods,
			reservedMilliCores: 60 + 10 + 5*2 + 2.5*4 + 400,
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.reservedMilliCores, calculateCPUReservedInMilliCPU(tc.coreNum, tc.maxPods))
	}
}
func Test_MultipathConfigOption_Case1(t *testing.T) {
	assert := require.New(t)
	config := NewHarvesterConfig()
	config.OS.ExternalStorage = ExternalStorageConfig{
		Enabled: true,
		MultiPathConfig: MultiPathConfig{
			Exceptions: []DiskConfig{
				{
					Vendor:  "DELL",
					Product: "DISK1",
				},
			},
		},
	}

	content, err := render("multipath.conf.tmpl", config.ExternalStorage.MultiPathConfig)
	assert.NoError(err, "expected no error while rendering multipath config")
	t.Log("rendered multipath config:")
	t.Log(content)

	expected := `blacklist {
}
blacklist_exceptions {
    device {
        vendor "DELL"
        product "DISK1"
    }
}
`

	assert.Equal(expected, content, "rendered multipath config should match expected output")
}

func Test_MultipathConfigOption_Case2(t *testing.T) {
	assert := require.New(t)
	config := NewHarvesterConfig()
	config.OS.ExternalStorage = ExternalStorageConfig{
		Enabled: true,
		MultiPathConfig: MultiPathConfig{
			BlackListWwid: []string{".*"},
			Blacklist:     []DiskConfig{},
			ExceptionWwid: []string{"^0QEMU_QEMU_HARDDISK_disk[0-9]+"},
			Exceptions: []DiskConfig{
				{
					Vendor:  "DELL",
					Product: "POWERVAULT",
				},
			},
		},
	}

	content, err := render("multipath.conf.tmpl", config.ExternalStorage.MultiPathConfig)
	assert.NoError(err, "expected no error while rendering multipath config with WWID and exceptions")

	t.Log("rendered multipath config with WWID and exceptions:")
	t.Log(content)

	expected := `blacklist {
    wwid ".*"
}
blacklist_exceptions {
    wwid "^0QEMU_QEMU_HARDDISK_disk[0-9]+"
    device {
        vendor "DELL"
        product "POWERVAULT"
    }
}
`

	assert.Equal(expected, content, "rendered multipath config should match expected output")
}

func Test_MultipathConfigOption_Case3(t *testing.T) {
	assert := require.New(t)
	config := NewHarvesterConfig()
	config.OS.ExternalStorage = ExternalStorageConfig{
		Enabled: true,
		MultiPathConfig: MultiPathConfig{
			BlackListWwid: []string{".*"},
			Blacklist: []DiskConfig{
				{
					Vendor:  "QEMU",
					Product: "QEMU HARDDISK",
				},
			},
			ExceptionWwid: []string{"^0QEMU_QEMU_HARDDISK_disk[0-9]+"},
			Exceptions: []DiskConfig{
				{
					Vendor:  "DELL",
					Product: "POWERVAULT",
				},
			},
		},
	}

	content, err := render("multipath.conf.tmpl", config.ExternalStorage.MultiPathConfig)
	assert.NoError(err, "expected no error while rendering multipath config with WWID and exceptions")

	t.Log("rendered multipath config with WWID and exceptions:")
	t.Log(content)

	expected := `blacklist {
    wwid ".*"
    device {
        vendor "QEMU"
        product "QEMU HARDDISK"
    }
}
blacklist_exceptions {
    wwid "^0QEMU_QEMU_HARDDISK_disk[0-9]+"
    device {
        vendor "DELL"
        product "POWERVAULT"
    }
}
`

	assert.Equal(expected, content, "rendered multipath config should match expected output")
}

func Test_MultipathConfigOption_Case4(t *testing.T) {
	assert := require.New(t)
	config := NewHarvesterConfig()
	config.OS.ExternalStorage = ExternalStorageConfig{
		Enabled:         true,
		MultiPathConfig: MultiPathConfig{},
	}

	content, err := render("multipath.conf.tmpl", config.ExternalStorage.MultiPathConfig)
	assert.NoError(err, "expected no error while rendering empty multipath config")

	t.Log("rendered empty multipath config:")
	t.Log(content)

	expected := `blacklist {
}
`

	assert.Equal(expected, content, "rendered empty multipath config should only contain empty blacklist section")
}

func Test_MultipathConfigOption_MultipleWwids(t *testing.T) {
	assert := require.New(t)
	config := NewHarvesterConfig()
	config.OS.ExternalStorage = ExternalStorageConfig{
		Enabled: true,
		MultiPathConfig: MultiPathConfig{
			BlackListWwid: []string{".*", "^36[0-9a-f]{30}"},
			ExceptionWwid: []string{"^0QEMU_QEMU_HARDDISK_disk[0-9]+", "^36001405[0-9a-f]{24}"},
		},
	}

	content, err := render("multipath.conf.tmpl", config.ExternalStorage.MultiPathConfig)
	assert.NoError(err, "expected no error while rendering multipath config with multiple WWIDs")

	t.Log("rendered multipath config with multiple WWIDs:")
	t.Log(content)

	expected := `blacklist {
    wwid ".*"
    wwid "^36[0-9a-f]{30}"
}
blacklist_exceptions {
    wwid "^0QEMU_QEMU_HARDDISK_disk[0-9]+"
    wwid "^36001405[0-9a-f]{24}"
}
`

	assert.Equal(expected, content, "rendered multipath config should match expected output with multiple WWIDs")
}

func Test_ToCosInstallEnv(t *testing.T) {
	hvConfig := NewHarvesterConfig()
	hvConfig.OS.ExternalStorage = ExternalStorageConfig{
		Enabled: true,
		MultiPathConfig: MultiPathConfig{
			Exceptions: []DiskConfig{
				{
					Vendor:  "DELL",
					Product: "DISK1",
				},
				{
					Vendor:  "HPE",
					Product: "DISK2",
				},
			},
		},
	}
	hvConfig.OS.AdditionalKernelArguments = "rd.iscsi.firmware rd.iscsi.ibft"
	assert := require.New(t)
	env, err := hvConfig.ToCosInstallEnv()
	assert.NoError(err)
	t.Log(env)

}
