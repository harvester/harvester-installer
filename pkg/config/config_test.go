package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarvesterConfig_sanitized(t *testing.T) {
	c := NewHarvesterConfig()
	c.Password = `#3tQ66t!`
	c.Token = `3mO3&nEJ`
	c.Wifi = []Wifi{{Name: "wifi1", Passphrase: `^s2I8Y2P`}}

	expected := NewHarvesterConfig()
	expected.Password = SanitizeMask
	expected.Token = SanitizeMask
	expected.Wifi = []Wifi{{Name: "wifi1", Passphrase: SanitizeMask}}

	s, err := c.sanitized()
	assert.Equal(t, nil, err)
	assert.Equal(t, expected, s)
}

func TestHarvesterConfig_GetKubeletLabelsArg(t *testing.T) {

	testCases := []struct {
		name      string
		input     map[string]string
		output    []string
		expectErr bool
	}{
		{
			name:   "Successfully creates node-labels argument",
			input:  map[string]string{"labelKey1": "value1"},
			output: []string{"node-labels=labelKey1=value1"},
		},
		{
			name:   "Returns nothing if no Labels is given",
			input:  map[string]string{},
			output: []string{},
		},
		{
			name:      "Error for invalid label name",
			input:     map[string]string{"???invalidName": "value"},
			output:    []string{},
			expectErr: true,
		},
		{
			name:      "Error for invalid label value",
			input:     map[string]string{"example.io/somelabel": "???value###NAH"},
			output:    []string{},
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c := NewHarvesterConfig()
			c.Labels = testCase.input

			result, err := c.GetKubeletArgs()

			if testCase.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t,
					testCase.output,
					result,
				)
			}
		})
	}
}
