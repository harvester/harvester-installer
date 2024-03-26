package preflight

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	// Constants here from Hardware Requirements in the documentation
	// https://docs.harvesterhci.io/v1.3/install/requirements/#hardware-requirements
	MinCPUTest         = 8
	MinCPUProd         = 16
	MinMemoryTest      = 32
	MinMemoryProd      = 64
	MinNetworkGbpsTest = 1
	MinNetworkGbpsProd = 10
)

var (
	// So that we can fake this stuff up for unit tests
	execCommand         = exec.Command
	procMemInfo         = "/proc/meminfo"
	devKvm              = "/dev/kvm"
	sysClassNetDevSpeed = "/sys/class/net/%s/speed"
)

// The Run() method of a preflight.Check returns a string.  If the string
// is empty, it means the check passed.  Otherwise, the string contains
// some text explaining why the check failed.  The error value will be set
// if the check itself failed to run at all for some reason.
type Check interface {
	Run() (string, error)
}

type CPUCheck struct{}
type MemoryCheck struct{}
type VirtCheck struct{}
type KVMHostCheck struct{}
type NetworkSpeedCheck struct {
	Dev string
}

func (c CPUCheck) Run() (msg string, err error) {
	out, err := execCommand("/usr/bin/nproc", "--all").Output()
	if err != nil {
		return
	}
	nproc, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if nproc < MinCPUTest {
		msg = fmt.Sprintf("Only %d CPU cores detected. Harvester requires at least %d cores for testing and %d for production use.",
			nproc, MinCPUTest, MinCPUProd)
	} else if nproc < MinCPUProd {
		msg = fmt.Sprintf("%d CPU cores detected. Harvester requires at least %d cores for production use.",
			nproc, MinCPUProd)
	}
	return
}

func (c MemoryCheck) Run() (msg string, err error) {
	meminfo, err := os.Open(procMemInfo)
	if err != nil {
		return
	}
	defer meminfo.Close()
	scanner := bufio.NewScanner(meminfo)
	var memTotalKiB int
	for scanner.Scan() {
		if n, _ := fmt.Sscanf(scanner.Text(), "MemTotal: %d kB", &memTotalKiB); n == 1 {
			break
		}
	}
	if memTotalKiB == 0 {
		err = errors.New("unable to extract MemTotal from /proc/cpuinfo")
		return
	}
	// MemTotal from /proc/cpuinfo is a bit less than the actual physical
	// memory in the system, due to reserved RAM not being included, so
	// we can't actually do a trivial check of MemTotalGiB < MinMemoryTest,
	// because it will fail.  For example:
	// - A host with 32GiB RAM may report MemTotal 32856636 = 31.11GiB
	// - A host with 64GiB RAM may report MemTotal 65758888 = 62.71GiB
	// - A host with 128GiB RAM may report MemTotal 131841120 = 125.73GiB
	// This means we have to test against a slightly lower number.  Knocking
	// 5% off is somewhat arbitrary but probably not unreasonable (e.g. for
	// 32GB we're actually allowing anything over 30.4GB, and for 64GB we're
	// allowing anything over 60.8GB).
	// Note that the above also means the warning messages below will be a
	// bit off (e.g. something like "System reports 31GiB RAM" on a 32GiB
	// system).
	memTotalGiB := memTotalKiB / (1 << 20)
	memReported := fmt.Sprintf("%dGiB", memTotalGiB)
	if memTotalGiB < 1 {
		// Just in case someone runs it on a really tiny VM...
		memReported = fmt.Sprintf("%dKiB", memTotalKiB)
	}
	if float32(memTotalGiB) < (MinMemoryTest * 0.95) {
		msg = fmt.Sprintf("Only %s RAM detected. Harvester requires at least %dGiB for testing and %dGiB for production use.",
			memReported, MinMemoryTest, MinMemoryProd)
	} else if float32(memTotalGiB) < (MinMemoryProd * 0.95) {
		msg = fmt.Sprintf("%s RAM detected. Harvester requires at least %dGiB for production use.",
			memReported, MinMemoryProd)
	}
	return
}

func (c VirtCheck) Run() (msg string, err error) {
	out, err := execCommand("/usr/bin/systemd-detect-virt", "--vm").Output()
	virt := strings.TrimSpace(string(out))
	if err != nil {
		// systemd-detect-virt will return a non-zero exit code
		// and print "none" if it doesn't detect a virtualization
		// environment.  The non-zero exit code manifests as a
		// non nil err here, so we have to handle that case and
		// return success from this check, because we're not
		// running virtualized.
		if virt == "none" {
			err = nil
		}
		return
	}
	msg = fmt.Sprintf("System is virtualized (%s) which is not supported in production.", virt)
	return
}

func (c KVMHostCheck) Run() (msg string, err error) {
	if _, err = os.Stat(devKvm); errors.Is(err, fs.ErrNotExist) {
		msg = "Harvester requires hardware-assisted virtualization, but /dev/kvm does not exist."
		err = nil
	}
	return
}

func (c NetworkSpeedCheck) Run() (msg string, err error) {
	speedPath := fmt.Sprintf(sysClassNetDevSpeed, c.Dev)
	out, err := os.ReadFile(speedPath)
	if err != nil {
		return
	}
	speedMbps, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if speedMbps < 1 {
		// speedMbps will be 0 if strconv.Atoi fails for some reason,
		// or -1 (if you can believe that) when using virtio NICs when
		// testing under virtualization.
		err = fmt.Errorf("unable to determine NIC speed from %s (got %d)", speedPath, speedMbps)
		return
	}
	// We need floats because 2.5Gbps ethernet is a thing.
	var speedGbps = float32(speedMbps) / 1000
	if speedGbps < MinNetworkGbpsTest {
		// Does anyone even _have_ < 1Gbps networking kit anymore?
		// Still, it's theoretically possible someone could have messed
		// up their switch config and be running 100Mbps...
		msg = fmt.Sprintf("Link speed of %s is only %dMpbs. Harvester requires at least %dGbps for testing and %dGbps for production use.",
			c.Dev, speedMbps, MinNetworkGbpsTest, MinNetworkGbpsProd)
	} else if speedGbps < MinNetworkGbpsProd {
		msg = fmt.Sprintf("Link speed of %s is %gGbps. Harvester requires at least %dGbps for production use.",
			c.Dev, speedGbps, MinNetworkGbpsProd)
	}
	return
}
