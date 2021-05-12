package config

import (
	k3os "github.com/rancher/k3os/pkg/config"
)

func ConvertToK3OS(config *HarvesterConfig) (*k3os.CloudConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
		return nil, err
	}

	cloudConfig := &k3os.CloudConfig{
		K3OS: k3os.K3OS{
			Install: &k3os.Install{},
		},
	}

	// top
	cloudConfig.K3OS.ServerURL = cfg.ServerURL
	cloudConfig.K3OS.Token = cfg.Token

	// OS
	cloudConfig.SSHAuthorizedKeys = cfg.OS.SSHAuthorizedKeys
	cloudConfig.WriteFiles = copyFiles(cfg.OS.WriteFiles)
	cloudConfig.Hostname = cfg.OS.Hostname
	cloudConfig.K3OS.Modules = cfg.OS.Modules
	cloudConfig.K3OS.Sysctls = cfg.OS.Sysctls
	cloudConfig.K3OS.NTPServers = cfg.OS.NTPServers
	cloudConfig.K3OS.DNSNameservers = cfg.OS.DNSNameservers
	cloudConfig.K3OS.Wifi = copyWifi(cfg.OS.Wifi)
	cloudConfig.K3OS.Password = cfg.OS.Password
	cloudConfig.K3OS.Environment = cfg.OS.Environment

	// OS.Install
	cloudConfig.K3OS.Install.ForceEFI = cfg.Install.ForceEFI
	cloudConfig.K3OS.Install.Device = cfg.Install.Device
	cloudConfig.K3OS.Install.Silent = cfg.Install.Silent
	cloudConfig.K3OS.Install.ISOURL = cfg.Install.ISOURL
	cloudConfig.K3OS.Install.PowerOff = cfg.Install.PowerOff
	cloudConfig.K3OS.Install.NoFormat = cfg.Install.NoFormat
	cloudConfig.K3OS.Install.Debug = cfg.Install.Debug
	cloudConfig.K3OS.Install.TTY = cfg.Install.TTY

	return cloudConfig, nil
}

func copyFiles(src []File) []k3os.File {
	if src == nil {
		return nil
	}
	r := make([]k3os.File, len(src))
	for i, element := range src {
		r[i].Encoding = element.Encoding
		r[i].Content = element.Content
		r[i].Owner = element.Owner
		r[i].Path = element.Path
		r[i].RawFilePermissions = element.RawFilePermissions
	}
	return r
}

func copyWifi(src []Wifi) []k3os.Wifi {
	if src == nil {
		return nil
	}
	r := make([]k3os.Wifi, len(src))
	for i, element := range src {
		r[i].Name = element.Name
		r[i].Passphrase = element.Passphrase
	}
	return r
}
