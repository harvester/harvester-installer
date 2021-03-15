package util

import (
	"io/ioutil"
	"os"
	"regexp"
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

	return data, nil
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
