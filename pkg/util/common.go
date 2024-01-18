package util

import (
	"regexp"
)

var (
	HarvesterNodeRoleLabelPrefix = "node-role.harvesterhci.io/"
	HarvesterWitnessNodeLabelKey = HarvesterNodeRoleLabelPrefix + "witness"
	HarvesterMgmtNodeLabelKey    = HarvesterNodeRoleLabelPrefix + "management"
	HarvesterWorkerNodeLabelKey  = HarvesterNodeRoleLabelPrefix + "worker"

	sizeRegexp = regexp.MustCompile(`^(\d+)(Mi|Gi)$`)
)

func StringSliceContains(sSlice []string, s string) bool {
	for _, target := range sSlice {
		if target == s {
			return true
		}
	}
	return false
}

func DupStrings(src []string) []string {
	if src == nil {
		return nil
	}
	s := make([]string, len(src))
	copy(s, src)
	return s
}

func ByteToGi(byte uint64) uint64 {
	return byte >> 30
}

func ByteToMi(byte uint64) uint64 {
	return byte >> 20
}

func GiToByte(gi uint64) uint64 {
	return gi << 30
}

func MiToByte(mi uint64) uint64 {
	return mi << 20
}
