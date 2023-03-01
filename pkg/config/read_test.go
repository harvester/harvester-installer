package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToEnv(t *testing.T) {
	obj := struct {
		PortNumber int    `json:"portNumber"`
		Enabled    bool   `json:"enabled"`
		Name       string `json:"name"`
	}{
		PortNumber: 8443,
		Enabled:    true,
		Name:       "harvester",
	}

	envs, err := ToEnv("PREFIX_", &obj)
	if err != nil {
		t.Fatalf("ToEnv() error = %v", err)
	}

	want := map[string]struct{}{
		"PREFIX_PORT_NUMBER=8443": {},
		"PREFIX_ENABLED=true":     {},
		"PREFIX_NAME=harvester":   {},
	}

	got := make(map[string]struct{})
	for _, env := range envs {
		got[env] = struct{}{}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %v, want %v", got, want)
	}
}

func TestReadUserData(t *testing.T) {
	config, err := readUserData("./testdata/userdata.yaml")
	assert.NoError(t, err, "expected no error during loading of userdata")
	assert.Equal(t, config.Token, "token", "expected token to be token")
	assert.Equal(t, config.OS.Password, "p@ssword", "expected password to be p@ssword")
	assert.Equal(t, config.Install.ManagementInterface.Method, "dhcp", "expected network mode to be dhcp")
}
