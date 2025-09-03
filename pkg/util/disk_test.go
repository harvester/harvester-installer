package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

const (
	sampleSerialDiskOutput = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "768.1M",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "serial-1",
         "children": [
            {
               "name": "0QEMU_QEMU_HARDDISK_serial-1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null
            }
         ]
      },{
         "name": "sdb",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "serial-1",
         "children": [
            {
               "name": "0QEMU_QEMU_HARDDISK_serial-1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null
            }
         ]
      },{
         "name": "sr0",
         "size": "5.8G",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      }
   ]
}
`

	reinstallDisks = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "3G",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "loop1",
         "size": "10G",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "10G",
         "type": "disk",
         "wwn": "0x60000000000000000e00000000010001",
         "serial": "beaf11",
         "children": [
            {
               "name": "sda1",
               "size": "2.5G",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda14",
               "size": "4M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda15",
               "size": "106M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            },{
               "name": "sda16",
               "size": "913M",
               "type": "part",
               "wwn": "0x60000000000000000e00000000010001",
               "serial": null
            }
         ]
      },{
         "name": "sr0",
         "size": "364K",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      },{
         "name": "vda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": null,
         "children": [
            {
               "name": "vda1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "vda6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            }
         ]
      }
   ]
}
`

	preInstalledMultiPath = `
{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "768.4M",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "disk1",
         "children": [
            {
               "name": "sda1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sda6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "0QEMU_QEMU_HARDDISK_disk1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "children": [
                  {
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part1",
                     "size": "1M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  }
               ]
            }
         ]
      },{
         "name": "sdb",
         "size": "250G",
         "type": "disk",
         "wwn": null,
         "serial": "disk1",
         "children": [
            {
               "name": "sdb1",
               "size": "1M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb2",
               "size": "50M",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb3",
               "size": "8G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb4",
               "size": "15G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb5",
               "size": "150G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "sdb6",
               "size": "76.9G",
               "type": "part",
               "wwn": null,
               "serial": null
            },{
               "name": "0QEMU_QEMU_HARDDISK_disk1",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "children": [
                  {
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part1",
                     "size": "1M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "0QEMU_QEMU_HARDDISK_disk1-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  }
               ]
            }
         ]
      },{
         "name": "sr0",
         "size": "5.8G",
         "type": "rom",
         "wwn": null,
         "serial": "QM00001"
      }
   ]
}
`
	raidDisks = `{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "780.5M",
         "type": "loop",
         "wwn": null,
         "serial": null
      },{
         "name": "sda",
         "size": "447.1G",
         "type": "disk",
         "wwn": "0x600508b1001cec28a12a38168f7bb195",
         "serial": "PDNMF0ARH1614W",
         "children": [
            {
               "name": "3600508b1001cec28a12a38168f7bb195",
               "size": "447.1G",
               "type": "mpath",
               "wwn": null,
               "serial": null
            }
         ]
      },{
         "name": "sdb",
         "size": "447.1G",
         "type": "disk",
         "wwn": "0x600508b1001c3e956986e526698cd830",
         "serial": "PDNMF0ARH1614W",
         "children": [
            {
               "name": "sdb1",
               "size": "1M",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "sdb2",
               "size": "50M",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "sdb3",
               "size": "8G",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "sdb4",
               "size": "15G",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "sdb5",
               "size": "170G",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "sdb6",
               "size": "254G",
               "type": "part",
               "wwn": "0x600508b1001c3e956986e526698cd830",
               "serial": null
            },{
               "name": "3600508b1001c3e956986e526698cd830",
               "size": "447.1G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "children": [
                  {
                     "name": "3600508b1001c3e956986e526698cd830-part1",
                     "size": "1M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "3600508b1001c3e956986e526698cd830-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "3600508b1001c3e956986e526698cd830-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "3600508b1001c3e956986e526698cd830-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "3600508b1001c3e956986e526698cd830-part5",
                     "size": "170G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  },{
                     "name": "3600508b1001c3e956986e526698cd830-part6",
                     "size": "254G",
                     "type": "part",
                     "wwn": null,
                     "serial": null
                  }
               ]
            }
         ]
      },{
         "name": "sr0",
         "size": "1024M",
         "type": "rom",
         "wwn": null,
         "serial": "475652914613"
      }
   ]
}`

	existingHarvesterInstalls = `{
   "blockdevices": [
      {
         "name": "loop0",
         "size": "4K",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop1",
         "size": "175.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop2",
         "size": "89.4M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop3",
         "size": "55.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop4",
         "size": "55.4M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop5",
         "size": "64M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop6",
         "size": "63.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop7",
         "size": "74.2M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop8",
         "size": "73.9M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop9",
         "size": "67.8M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop10",
         "size": "67.8M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop11",
         "size": "374.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop12",
         "size": "375.1M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop13",
         "size": "349.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop14",
         "size": "349.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop15",
         "size": "504.2M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop16",
         "size": "505.1M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop17",
         "size": "273.6M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop18",
         "size": "273M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop19",
         "size": "91.7M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop20",
         "size": "44.3M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop21",
         "size": "87M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop22",
         "size": "38.8M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop24",
         "size": "6.8M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop25",
         "size": "6.8M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "loop26",
         "size": "172.3M",
         "type": "loop",
         "wwn": null,
         "serial": null,
         "label": null
      },{
         "name": "sda",
         "size": "838.1G",
         "type": "disk",
         "wwn": "0x600508b1001cc488149adefce15584da",
         "serial": "PDNLH0BRH7T0VY",
         "label": null,
         "children": [
            {
               "name": "sda1",
               "size": "1G",
               "type": "part",
               "wwn": "0x600508b1001cc488149adefce15584da",
               "serial": null,
               "label": null
            },{
               "name": "sda2",
               "size": "2G",
               "type": "part",
               "wwn": "0x600508b1001cc488149adefce15584da",
               "serial": null,
               "label": null
            },{
               "name": "sda3",
               "size": "835G",
               "type": "part",
               "wwn": "0x600508b1001cc488149adefce15584da",
               "serial": null,
               "label": null,
               "children": [
                  {
                     "name": "ubuntu--vg-ubuntu--lv",
                     "size": "835G",
                     "type": "lvm",
                     "wwn": null,
                     "serial": null,
                     "label": null
                  }
               ]
            }
         ]
      },{
         "name": "sdb",
         "size": "3.8G",
         "type": "disk",
         "wwn": null,
         "serial": "General_-0:0",
         "label": null,
         "children": [
            {
               "name": "sdb1",
               "size": "3.8G",
               "type": "part",
               "wwn": null,
               "serial": null,
               "label": null
            }
         ]
      },{
         "name": "sdc",
         "size": "250G",
         "type": "disk",
         "wwn": "0x60000000000000000e00000000010001",
         "serial": "1001",
         "label": null,
         "children": [
            {
               "name": "mpatha",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "label": null,
               "children": [
                  {
                     "name": "mpatha-part1",
                     "size": "64M",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_GRUB"
                  },{
                     "name": "mpatha-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_OEM"
                  },{
                     "name": "mpatha-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_RECOVERY"
                  },{
                     "name": "mpatha-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_STATE"
                  },{
                     "name": "mpatha-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_PERSISTENT"
                  },{
                     "name": "mpatha-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "HARV_LH_DEFAULT"
                  }
               ]
            }
         ]
      },{
         "name": "sdd",
         "size": "250G",
         "type": "disk",
         "wwn": "0x60000000000000000e00000000010001",
         "serial": "1001",
         "label": null,
         "children": [
            {
               "name": "mpatha",
               "size": "250G",
               "type": "mpath",
               "wwn": null,
               "serial": null,
               "label": null,
               "children": [
                  {
                     "name": "mpatha-part1",
                     "size": "64M",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_GRUB"
                  },{
                     "name": "mpatha-part2",
                     "size": "50M",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_OEM"
                  },{
                     "name": "mpatha-part3",
                     "size": "8G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_RECOVERY"
                  },{
                     "name": "mpatha-part4",
                     "size": "15G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_STATE"
                  },{
                     "name": "mpatha-part5",
                     "size": "150G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "COS_PERSISTENT"
                  },{
                     "name": "mpatha-part6",
                     "size": "76.9G",
                     "type": "part",
                     "wwn": null,
                     "serial": null,
                     "label": "HARV_LH_DEFAULT"
                  }
               ]
            }
         ]
      },{
         "name": "nvme0n1",
         "size": "1.8T",
         "type": "disk",
         "wwn": "eui.0025385c2140432c",
         "serial": "S6S2NS0TC11162K",
         "label": null
      }
   ]
}`
)

func Test_identifyUniqueDisksWithSerialNumber(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(sampleSerialDiskOutput))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 1, "expected to find 1 disk only")
}

func Test_identifyUniqueDisksWithExistingData(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(reinstallDisks))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 2, "expected to find 2 disks only")
}

func Test_identifyUniqueDisksOnExistingInstalls(t *testing.T) {
	assert := require.New(t)
	result, err := identifyUniqueDisks([]byte(preInstalledMultiPath))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(result, 1, "expected to find 1 disk only")
}

func Test_identifyUniqueDisks(t *testing.T) {
	assert := require.New(t)
	out, err := identifyUniqueDisks([]byte(raidDisks))
	assert.NoError(err, "expected no error while parsing disk data")
	t.Log(out)
}

func Test_identifyUniqueDisksWithHarvesterInstall(t *testing.T) {
	assert := require.New(t)
	results, err := identifyUniqueDisksWithHarvesterInstall([]byte(existingHarvesterInstalls))
	assert.NoError(err, "expected no error while parsing disk data")
	assert.Len(results, 1, "expected to find 1 disk from sample data")
}
