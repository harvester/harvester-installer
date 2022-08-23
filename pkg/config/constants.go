package config

const (
	ModeCreate  = "create"
	ModeJoin    = "join"
	ModeUpgrade = "upgrade"

	NetworkMethodDHCP   = "dhcp"
	NetworkMethodStatic = "static"
	NetworkMethodNone   = "none"

	MgmtInterfaceName     = "mgmt-br"
	MgmtBondInterfaceName = "mgmt-bo"

	RancherdConfigFile = "/etc/rancher/rancherd/config.yaml"
)
