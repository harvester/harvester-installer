package util

import (
	"fmt"
	"testing"

	"github.com/rancher/mapper/values"
	"github.com/stretchr/testify/assert"
)

func Test_parseCmdLineWithPrefix(t *testing.T) {
	cmdline := `x y harvester.a.b=true "harvester.c=d" harvester.e harvester.f=1 harvester.f=2`
	m, err := parseCmdLine(cmdline, "harvester")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"a": map[string]interface{}{"b": "true"},
		"c": "d",
		"e": "true",
		"f": []string{"1", "2"},
	}
	assert.Equal(t, want, m)
}

func Test_parseCmdLineWithoutPrefix(t *testing.T) {
	cmdline := `mode=live console=tty1 console=ttyS0,115200n8`
	m, err := parseCmdLine(cmdline, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"mode":    "live",
		"console": []string{"tty1", "ttyS0,115200n8"},
	}
	assert.Equal(t, want, m)
}

func Test_parseCmdLineWithNetworkInterface(t *testing.T) {

	cmdline := `harvester.os.sshAuthorizedKeys=a  harvester.install.management_interface.method=dhcp harvester.install.management_interface.bond_options.mode=balance-tlb harvester.install.management_interface.bond_options.miimon=100 harvester.os.sshAuthorizedKeys=b harvester.install.mode=create harvester.install.management_interface.interfaces="hwAddr: ab:cd:ef:gh:ij:kl" harvester.install.management_interface.interfaces="hwAddr:   de:fg:hi:jk:lm:no" harvester.install.management_interface.interfaces="ens3" harvester.install.management_interface.interfaces="name:ens5"`

	m, err := parseCmdLine(cmdline, "harvester")
	if err != nil {
		t.Fatal(err)
	}

	want := []interface{}{
		map[string]interface{}{"hwAddr": "ab:cd:ef:gh:ij:kl"},
		map[string]interface{}{"hwAddr": "de:fg:hi:jk:lm:no"},
		map[string]interface{}{"name": "ens3"},
		map[string]interface{}{"name": "ens5"},
	}

	have, ok := values.GetValue(m, "install", "management_interface", "interfaces")
	if !ok {
		t.Fatal(fmt.Errorf("no network interfaces found"))
	}

	assert.Equal(t, want, have)
}

func Test_parseCmdLineWithSchemeVersion(t *testing.T) {
	cmdline := `harvester.os.sshAuthorizedKeys=a  harvester.install.management_interface.method=dhcp harvester.install.management_interface.bond_options.mode=balance-tlb harvester.install.management_interface.bond_options.miimon=100 harvester.os.sshAuthorizedKeys=b harvester.install.mode=create harvester.install.management_interface.interfaces="hwAddr: ab:cd:ef:gh:ij:kl" harvester.install.management_interface.interfaces="hwAddr:   de:fg:hi:jk:lm:no" harvester.scheme_version=1`

	m, err := parseCmdLine(cmdline, "harvester")
	assert.NoError(t, err, "expected no error while parsing arguments")

	val, ok := m["scheme_version"]
	assert.True(t, ok, "expected to find key scheme_version")
	var tmp uint64
	assert.IsType(t, tmp, val, "expected to find scheme_version to be type uint")
}
