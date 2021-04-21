package config

import (
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
	schema.Mapper.ToInternal(data)
	return *result, convert.ToObj(data, result)
}
