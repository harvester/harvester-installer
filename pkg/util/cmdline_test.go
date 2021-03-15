package util

import (
	"testing"

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
