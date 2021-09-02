package console

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var standardWickedOutput = `
Notice: eth0: Request to acquire DHCPv4 lease with UUID 25273861-5377-0900-a71e-000001000000
INTERFACE='eth0'
TYPE='dhcp'
FAMILY='ipv4'
UUID='25273861-5377-0900-a71e-000001000000'
IPADDR='172.16.4.4/16'
NETMASK='255.255.0.0'
NETWORK='172.16.0.0'
PREFIXLEN='16'
GATEWAYS='172.16.0.1'
DNSSERVERS='114.114.114.114'
CLIENTID='ff:5e:29:b3:1e:00:01:00:01:28:c8:44:d0:e0:d5:5e:29:b3:1e'
SERVERID='172.16.0.1'
SENDERHWADDR='4c:e9:e4:72:63:9c'
ACQUIRED='1631069989'
LEASETIME='86400'
RENEWALTIME='43200'
REBINDTIME='75600'
BOOTSERVERADDR='172.16.0.1'
`

var simplestValidOutput = `
IPADDR='172.16.4.4/16'
`

func TestParseIPFromWickedOutput(t *testing.T) {
	_, err := parseIPFromWickedOutput([]byte(standardWickedOutput))
	assert.Nil(t, err)

	_, err = parseIPFromWickedOutput([]byte(simplestValidOutput))
	assert.Nil(t, err)
}
