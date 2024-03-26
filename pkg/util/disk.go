package util

import (
	"fmt"
	"strconv"
)

const (
	MinDiskSize       = 250 << 30
	MinPersistentSize = 150 << 30
	MiByteMultiplier  = 1 << 20
	GiByteMultiplier  = 1 << 30

	// 50Mi for COS_OEM, 15Gi for COS_STATE, 8Gi for COS_RECOVERY, 64Mi for ESP partition, 50Gi for VM data
	fixedOccupiedSize = (50 + 15360 + 8192 + 64 + 51200) * MiByteMultiplier
)

func ParsePartitionSize(diskSizeBytes uint64, partitionSize string) (uint64, error) {
	if diskSizeBytes < MinDiskSize {
		return 0, fmt.Errorf("Installation disk size is too small. Minimum %dGi is required", ByteToGi(MinDiskSize))
	}
	actualDiskSizeBytes := diskSizeBytes - fixedOccupiedSize

	if !sizeRegexp.MatchString(partitionSize) {
		return 0, fmt.Errorf("Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed")
	}

	size, err := strconv.ParseUint(partitionSize[:len(partitionSize)-2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse partition size: %s", partitionSize)
	}

	var partitionBytes uint64
	unit := partitionSize[len(partitionSize)-2:]
	switch unit {
	case "Mi":
		partitionBytes = size * MiByteMultiplier
	case "Gi":
		partitionBytes = size * GiByteMultiplier
	}

	if partitionBytes < MinPersistentSize {
		return 0, fmt.Errorf("Partition size is too small. Minimum %dGi is required", ByteToGi(MinPersistentSize))
	}
	if partitionBytes > actualDiskSizeBytes {
		return 0, fmt.Errorf("Partition size is too large. Maximum %dGi is allowed", ByteToGi(actualDiskSizeBytes))
	}

	return partitionBytes, nil
}
