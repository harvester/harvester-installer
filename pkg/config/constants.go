package config

const (
	ModeCreate  = "create"
	ModeJoin    = "join"
	ModeUpgrade = "upgrade"

	NetworkMethodDHCP   = "dhcp"
	NetworkMethodStatic = "static"
	NetworkMethodNone   = "none"

	MgmtInterfaceName     = "br-mgmt"
	MgmtBondInterfaceName = "bond-mgmt"

	RancherdConfigFile = "/etc/rancher/rancherd/config.yaml"
)
