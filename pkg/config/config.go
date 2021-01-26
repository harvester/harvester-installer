package config

import (
	"github.com/rancher/k3os/pkg/config"
)

var (
	Config = InstallConfig{}
)

type InstallConfig struct {
	config.CloudConfig

	ExtraK3sArgs []string
	InstallMode  string
}
