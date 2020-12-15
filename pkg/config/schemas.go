package config

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/rancher/k3os/pkg/config"
	"github.com/rancher/mapper"
	"github.com/rancher/mapper/convert"
)

var (
	schemas = mapper.NewSchemas().Init(func(s *mapper.Schemas) *mapper.Schemas {
		s.DefaultMappers = func() []mapper.Mapper {
			return []mapper.Mapper{
				config.NewToMap(),
				config.NewToSlice(),
				config.NewToBool(),
				&config.FuzzyNames{},
			}
		}
		return s
	}).MustImport(config.CloudConfig{})
	schema = schemas.Schema("cloudConfig")
)

func ToCloudConfig(yamlBytes []byte) (*config.CloudConfig, error) {
	result := &config.CloudConfig{}
	data := map[string]interface{}{}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return result, fmt.Errorf("failed to unmarshal yaml: %v", err)
	}
	schema.Mapper.ToInternal(data)
	return result, convert.ToObj(data, result)
}
