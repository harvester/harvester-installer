package console

const (
	titlePanel             = "title"
	debugPanel             = "debug"
	diskPanel              = "disk"
	dataDiskPanel          = "dataDisk"
	dataDiskValidatorPanel = "dataDiskValidator"
	askForceMBRTitlePanel  = "askForceMBRTitle"
	askForceMBRPanel       = "askForceMBR"
	forceMBRNotePanel      = "forceMBRNote"
	askCreatePanel         = "askCreate"
	serverURLPanel         = "serverUrl"
	passwordPanel          = "osPassword"
	passwordConfirmPanel   = "osPasswordConfirm"
	sshKeyPanel            = "sshKey"
	tokenPanel             = "token"
	proxyPanel             = "proxy"
	askInterfacePanel      = "askInterface"
	askVlanIDPanel         = "askVlanID"
	askBondModePanel       = "askBondMode"
	bondNotePanel          = "bondNote"
	askNetworkMethodPanel  = "askNetworkMethod"
	hostNamePanel          = "hostname"
	addressPanel           = "address"
	gatewayPanel           = "gateway"
	dnsServersPanel        = "dnsServers"
	networkValidatorPanel  = "networkValidator"
	diskValidatorPanel     = "diskValidator"
	cloudInitPanel         = "cloudInit"
	validatorPanel         = "validator"
	notePanel              = "note"
	installPanel           = "install"
	footerPanel            = "footer"
	spinnerPanel           = "spinner"
	confirmInstallPanel    = "confirmInstall"
	confirmUpgradePanel    = "confirmUpgrade"
	upgradePanel           = "upgrade"
	askVipMethodPanel      = "askVipMethodPanel"
	vipPanel               = "vipPanel"
	vipTextPanel           = "vipTextPanel"
	ntpServersPanel        = "ntpServersPanel"

	networkTitle          = "Configure network connection"
	askBondModeLabel      = "Bond Mode"
	askInterfaceLabel     = "Management NIC"
	askVlanIDLabel        = "VLAN ID"
	askNetworkMethodLabel = "IPv4 Method"
	hostNameLabel         = "HostName"
	addressLabel          = "IPv4 Address"
	gatewayLabel          = "Gateway"
	dnsServersLabel       = "DNS Servers"
	ntpServersLabel       = "NTP Servers"

	networkMethodDHCPText   = "Automatic (DHCP)"
	networkMethodStaticText = "Static"

	vipTitle          = "Configure VIP"
	vipLabel          = "VIP"
	askVipMethodLabel = "VIP Mode"

	clusterTokenCreateNote = "Note: The token is used for adding nodes to the cluster"
	clusterTokenJoinNote   = "Note: Input the token of the existing cluster"
	serverURLNote          = "Note: Input VIP/domain name of the management node"
	proxyNote              = "Note: In the form of \"http://[[user][:pass]@]host[:port]/\"."
	sshKeyNote             = "For example: https://github.com/<username>.keys"
	ntpServersNote         = "Note: It's recommended to configure NTP servers to make sure the time is synced among all nodes. You can use comma to add more NTP servers."
	dnsServersNote         = "Note: You can use comma to add more DNS servers. Leave blank to use default DNS."
	bondNote               = "Note: Select one or more NICs for the Management NIC.\nUse the default value for the Bond Mode if only one NIC is selected."
	forceMBRNote           = "Note: GPT is used by default. You can use MBR if you encountered compatibility issues."

	authorizedFile = "/home/rancher/.ssh/authorized_keys"
)
