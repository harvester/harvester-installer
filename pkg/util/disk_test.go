package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePartitionSize(t *testing.T) {
	testCases := []struct {
		diskSize      uint64
		partitionSize string
		result        uint64
		err           string
	}{
		{
			diskSize:      300 * GiByteMultiplier,
			partitionSize: "150Gi",
			result:        150 * GiByteMultiplier,
		},
		{
			diskSize:      500 * GiByteMultiplier,
			partitionSize: "153600Mi",
			result:        153600 * MiByteMultiplier,
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "1999Gi",
			err:           "Partition size is too large. Maximum 1926Gi is allowed",
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "0Gi",
			err:           "Partition size is too small. Minimum 150Gi is required",
		},
		{
			diskSize:      500 * GiByteMultiplier,
			partitionSize: "0Mi",
			err:           "Partition size is too small. Minimum 150Gi is required",
		},
		{
			diskSize:      100 * GiByteMultiplier,
			partitionSize: "50Gi",
			err:           "Installation disk size is too small. Minimum 250Gi is required",
		},
		{
			diskSize:      249 * GiByteMultiplier,
			partitionSize: "50Gi",
			err:           "Installation disk size is too small. Minimum 250Gi is required",
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "abcd",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "1Ti",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "50Ki",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      2000 * GiByteMultiplier,
			partitionSize: "5.5",
			err:           "Partition size must end with 'Mi' or 'Gi'. Decimals and negatives are not allowed",
		},
		{
			diskSize:      400 * GiByteMultiplier,
			partitionSize: "385933Mi",
			err:           "Partition size is too large. Maximum 326Gi is allowed",
		},
	}

	for _, tc := range testCases {
		result, err := ParsePartitionSize(tc.diskSize, tc.partitionSize)
		assert.Equal(t, tc.result, result)
		if err != nil {
			assert.EqualError(t, err, tc.err)
		}
	}
}
