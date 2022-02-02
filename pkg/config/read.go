package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/mapper/convert"

	"github.com/harvester/harvester-installer/pkg/util"
)

const (
	kernelParamPrefix = "harvester"
)

// ReadConfig constructs a config by reading various sources
func ReadConfig() (HarvesterConfig, error) {
	result := NewHarvesterConfig()
	data, err := util.ReadCmdline(kernelParamPrefix)
	if err != nil {
		return *result, err
	}
	err = ToNetworkInterface(data)
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

func ToNetworkInterface(data map[string]interface{}) error {
	if installInterface, ok := data["install"]; ok {
		if networksInterface, ok := installInterface.(map[string]interface{})["networks"]; ok {
			if mgmtInterface, ok := networksInterface.(map[string]interface{})["harvester-mgmt"]; ok {
				if interfaces, ok := mgmtInterface.(map[string]interface{})["interfaces"]; ok {
					var ifDetails []string
					var outDetails []NetworkInterface
					switch interfaces.(type) {
					case string:
						ifDetails = append(ifDetails, interfaces.(string))
					case []string:
						ifDetails = interfaces.([]string)
					}
					for _, v := range ifDetails {
						tmpStrings := strings.SplitN(v, ":", 2)
						n := NetworkInterface{}
						err := json.Unmarshal([]byte(fmt.Sprintf("{\"%s\":\"%s\"}", tmpStrings[0], strings.ReplaceAll(tmpStrings[1], " ", ""))), &n)
						if err != nil {
							return err
						}
						outDetails = append(outDetails, n)
					}
					mgmtInterface.(map[string]interface{})["interfaces"] = outDetails
				}
			}
		}
	}
	return nil
}
