package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/rancher/mapper/convert"

	"github.com/harvester/harvester-installer/pkg/util"
)

const (
	kernelParamPrefix   = "harvester"
	defaultUserDataFile = "/oem/userdata.yaml"
)

// ReadConfig constructs a config by reading various sources
func ReadConfig() (HarvesterConfig, error) {
	result := NewHarvesterConfig()
	data, err := util.ReadCmdline(kernelParamPrefix)
	if err != nil {
		return *result, err
	}
	schema.Mapper.ToInternal(data)
	return *result, convert.ToObj(data, result)
}

func ToEnv(prefix string, obj interface{}) ([]string, error) {
	data, err := convert.EncodeToMap(obj)
	if err != nil {
		return nil, err
	}

	return mapToEnv(prefix, data), nil
}

func mapToEnv(prefix string, data map[string]interface{}) []string {
	var result []string
	for k, v := range data {
		keyName := strings.ToUpper(prefix + convert.ToYAMLKey(k))
		if data, ok := v.(map[string]interface{}); ok {
			subResult := mapToEnv(keyName+"_", data)
			result = append(result, subResult...)
		} else {
			result = append(result, fmt.Sprintf("%s=%v", keyName, v))
		}
	}
	return result
}

// ReadUserDataConfig constructs a config from userdata
func ReadUserDataConfig() (HarvesterConfig, error) {
	return readUserData(defaultUserDataFile)
}

func readUserData(fileName string) (HarvesterConfig, error) {
	result := NewHarvesterConfig()
	_, err := os.Stat(fileName)

	if os.IsNotExist(err) {
		return *result, nil
	} else if err != nil {
		return *result, err
	}

	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return *result, err
	}

	tidyContents, err := cleanupFile(contents)
	if err != nil {
		return *result, err
	}
	result, err = LoadHarvesterConfig(tidyContents)
	return *result, err
}

func cleanupFile(content []byte) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	var config []string
	for _, v := range lines {
		if v != "#cloud-config" && v != "#!cloud-config" {
			config = append(config, v)
		}
	}

	return []byte(strings.Join(config, "\n")), nil
}
