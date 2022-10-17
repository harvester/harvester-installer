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
	var ifDetails []string
	var outDetails []interface{}

	switch networkInterfaces.(type) {
	case string:
		ifDetails = append(ifDetails, networkInterfaces.(string))
	case []string:
		ifDetails = networkInterfaces.([]string)
	}
	for _, v := range ifDetails {
		tmpStrings := strings.SplitN(v, ":", 2)
		n := make(map[string]interface{})
		err := json.Unmarshal([]byte(fmt.Sprintf("{\"%s\":\"%s\"}", tmpStrings[0], strings.ReplaceAll(tmpStrings[1], " ", ""))), &n)
		if err != nil {
			return err
		}
		outDetails = append(outDetails, n)
	}

	values.PutValue(data, outDetails, "install", "management_interface", "interfaces")
	return nil
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
