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

	DefaultCosOemSizeMiB      = 50
	DefaultCosStateSizeMiB    = 15360
	DefaultCosRecoverySizeMiB = 8192

	DefaultPersistentPercentageNum = 0.3
	PersistentSizeMinGiB           = 150
)
