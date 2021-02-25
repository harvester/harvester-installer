package util

import (
	"io/ioutil"
	"path"
	"testing"
)

// LoadFixture loads a testing fixture from testdata dir
func LoadFixture(t *testing.T, name string) []byte {
	path := path.Join("testdata", name)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Fail to load fixture %q", path)
	}
	return data
}
