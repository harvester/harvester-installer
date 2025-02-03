package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/rancher/mapper/values"
)

// parse kernel parameters
func parseCmdLine(cmdline string, prefix string) (map[string]interface{}, error) {
	//supporting regex https://regexr.com/4mq0s
	parser, err := regexp.Compile(`(\"[^\"]+\")|([^\s]+=(\"[^\"]+\")|([^\s]+))`)
	if err != nil {
		return nil, nil
	}

	data := map[string]interface{}{}
	for _, item := range parser.FindAllString(cmdline, -1) {
		parts := strings.SplitN(item, "=", 2)
		value := "true"
		if len(parts) > 1 {
			value = strings.Trim(parts[1], `"`)
		}
		keys := strings.Split(strings.Trim(parts[0], `"`), ".")
		if prefix != "" {
			if keys[0] != prefix {
				continue
			}
			keys = keys[1:]
		}
		existing, ok := values.GetValue(data, keys...)
		if ok {
			switch v := existing.(type) {
			case string:
				values.PutValue(data, []string{v, value}, keys...)
			case []string:
				values.PutValue(data, append(v, value), keys...)
			}
		} else {
			values.PutValue(data, value, keys...)
		}
	}

	err = toNetworkInterfaces(data)
	if err != nil {
		return data, err
	}
	err = toSchemeVersion(data)
	return data, err
}

// ReadCmdline parses /proc/cmdline and returns a map contains kernel parameters
func ReadCmdline(prefix string) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile("/proc/cmdline")
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return parseCmdLine(string(bytes), prefix)
}

// parse kernel arguments and process network interfaces as a struct
func toNetworkInterfaces(data map[string]interface{}) error {
	networkInterfaces, ok := values.GetValue(data, "install", "management_interface", "interfaces")
	if !ok {
		return nil
	}
	ifDetails := make([]string, 0)

	switch networkInterfaces.(type) {
	case string:
		ifDetails = append(ifDetails, networkInterfaces.(string))
	case []string:
		ifDetails = networkInterfaces.([]string)
	}

	outDetails := make([]interface{}, 0, len(ifDetails))
	for _, v := range ifDetails {
		n, err := parseIfDetails(v)
		if err != nil {
			return err
		}
		outDetails = append(outDetails, *n)
	}

	values.PutValue(data, outDetails, "install", "management_interface", "interfaces")
	return nil
}

// parseIfDetails accepts strings in the form of:
// - "hwAddr: ab:cd:ef:gh:ij:kl"
// - "name: ens3"
// - "ab:cd:ef:gh:ij:kl"
// - "ens3"
// and returns a map of either
// "hwAddr: ab:cd:ef:gh:ij:kl"
// or
// "name: ens3"
func parseIfDetails(details string) (*map[string]interface{}, error) {
	var (
		parts []string
		data  string
	)

	for _, s := range strings.Split(details, ":") {
		parts = append(parts, strings.TrimSpace(s))
	}

	switch len(parts) {
	case 7:
		// hwAddr: ab:cd:ef:gh:ij:kl
		if parts[0] != "hwAddr" {
			return nil, fmt.Errorf("could not parse interface details %v", details)
		}
		data = fmt.Sprintf("{\"hwAddr\":\"%v\"}", strings.Join(parts[1:], ":"))
	case 6:
		// ab:cd:ef:gh:ij:kl
		data = fmt.Sprintf("{\"hwAddr\":\"%v\"}", strings.Join(parts, ":"))
	case 2:
		// name: ens3
		if parts[0] != "name" {
			return nil, fmt.Errorf("could not parse interface details %v", details)
		}
		data = fmt.Sprintf("{\"name\":\"%v\"}", parts[1])
	case 1:
		// ens3
		data = fmt.Sprintf("{\"name\":\"%v\"}", parts[0])
	default:
		return nil, fmt.Errorf("could not parse interface details %v", details)
	}

	n := make(map[string]interface{})
	err := json.Unmarshal([]byte(data), &n)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func toSchemeVersion(data map[string]interface{}) error {
	schemeVersion, ok := values.GetValue(data, "scheme_version")
	if !ok {
		return nil
	}

	schemeVersionUint, err := strconv.ParseUint(schemeVersion.(string), 10, 32)
	if err != nil {
		return err
	}
	values.PutValue(data, schemeVersionUint, "scheme_version")
	return nil
}
