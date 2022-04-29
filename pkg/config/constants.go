package config

const (
	ModeCreate  = "create"
	ModeJoin    = "join"
	ModeUpgrade = "upgrade"
	ModeInstall = "install"

	NetworkMethodDHCP   = "dhcp"
	NetworkMethodStatic = "static"
	NetworkMethodNone   = "none"

	MgmtInterfaceName = "harvester-mgmt"

	RancherdConfigFile = "/etc/rancher/rancherd/config.yaml"
)
