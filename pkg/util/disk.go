package util

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

const (
	MinDiskSize       = 250 << 30
	MinPersistentSize = 150 << 30
	MiByteMultiplier  = 1 << 20
	GiByteMultiplier  = 1 << 30

	// 50Mi for COS_OEM, 15Gi for COS_STATE, 8Gi for COS_RECOVERY, 64Mi for ESP partition, 50Gi for VM data
	fixedOccupiedSize = (50 + 15360 + 8192 + 64 + 51200) * MiByteMultiplier

	diskType            = "disk"
	PartitionType       = "part"
	MpathType           = "mpath"
	CosDiskLabelPrefix  = "COS_OEM"
)

// internal objects to parse lsblk output
type BlockDevices struct {
	Disks []Device `json:"blockdevices"`
}

type Device struct {
	Name     string   `json:"name"`
	Size     string   `json:"size"`
	DiskType string   `json:"type"`
	WWN      string   `json:"wwn,omitempty"`
	Serial   string   `json:"serial,omitempty"`
	Label    string   `json:"label,omitempty"`
	Children []Device `json:"children,omitempty"`
}

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

func GetUniqueDisks() ([]Device, error) {
	output, err := exec.Command("/bin/sh", "-c", `lsblk -J -o NAME,SIZE,TYPE,WWN,SERIAL,LABEL`).CombinedOutput()
	if err != nil {
		return nil, err
	}

	return identifyUniqueDisks(output)
}

// identifyUniqueDisks parses the json output of lsblk and identifies
// unique disks by comparing their serial number info and wwn details
func identifyUniqueDisks(output []byte) ([]Device, error) {

	resultMap, err := filterUniqueDisks(output)
	if err != nil {
		return nil, err
	}

	returnDisks := make([]Device, 0, len(resultMap))
	// generate list of disks
	for _, v := range resultMap {
		returnDisks = append(returnDisks, v)
	}

	return returnDisks, nil
}

// filterUniqueDisks will dedup results of disk output to generate a map[disName]Device of unique devices
func filterUniqueDisks(output []byte) (map[string]Device, error) {
	disks := &BlockDevices{}
	err := json.Unmarshal(output, disks)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling lsblk json output: %v", err)
	}
	// identify devices which may be unique
	dedupMap := make(map[string]Device)
	for _, disk := range disks.Disks {
		if disk.DiskType == diskType {
			// no serial or wwn info present
			// add to list of disks
			if disk.WWN == "" && disk.Serial == "" {
				dedupMap[disk.Name] = disk
				continue
			}

			if disk.Serial != "" {
				_, ok := dedupMap[disk.Serial]
				if !ok {
					dedupMap[disk.Serial] = disk
				}
			}

			// disks may have same serial number but different wwn when used with a raid array
			// as evident in test data from a host with a raid array
			// in this case if serial number is same, we still check for unique wwn
			if disk.WWN != "" {
				_, ok := dedupMap[disk.WWN]
				if !ok {
					dedupMap[disk.WWN] = disk
				}
				continue
			}
		}
	}
	// devices may appear twice in the map when both serial number and wwn info is present
	// we need to ensure only unique names are shown in the console
	resultMap := make(map[string]Device)
	for _, v := range dedupMap {
		resultMap[v.Name] = v
	}

	return resultMap, nil
}

func GetUniqueDisksWithHarvesterInstall() ([]Device, error) {
	output, err := exec.Command("/bin/sh", "-c", `lsblk -J -o NAME,SIZE,TYPE,WWN,SERIAL,LABEL`).CombinedOutput()
	if err != nil {
		return nil, err
	}

	return identifyUniqueDisksWithHarvesterInstall(output)
}

// identifyUniqueDisksWithHarvesterInstall will identify disks which may already be in use with old Harvester
// installs. This is done by check if a label with prefix COS exists on any of the partitions
// and only those disks are returned for getWipeDiskOptions
func identifyUniqueDisksWithHarvesterInstall(output []byte) ([]Device, error) {
	disks, err := identifyUniqueDisks(output)
	if err != nil {
		return nil, err
	}

	var returnedDisks []Device
	for _, d := range disks {
		if deviceContainsCOSPartition(d) {
			returnedDisks = append(returnedDisks, d)
		}
	}
	return returnedDisks, nil
}

func deviceContainsCOSPartition(disk Device) bool {
	for _, partition := range disk.Children {
		if partition.DiskType == MpathType {
			return deviceContainsCOSPartition(partition)
		}
		if partition.DiskType == PartitionType && partition.Label == CosDiskLabelPrefix {
			return true
		}
	}
	return false
}
