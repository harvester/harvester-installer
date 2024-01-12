harvester-installer
========
[![Build Status](https://drone-publish.rancher.io/api/badges/harvester/harvester-installer/status.svg)](https://drone-publish.rancher.io/harvester/harvester-installer)

Repo for building the [Harvester](https://github.com/harvester/harvester)
ISO image.  This includes the various scripts necessary to build the ISO
itself, plus the `harvester-installer` binary and related scripts that
perform system installation when the ISO is booted.

## Building

To build an ISO image, run:

`make`

This will:

1. Build the `harvester-installer` binary.
2. Create an archive of all the necessary Harvester and Rancher charts
   and container images.
3. Create the `harvester-cluster-repo` container image, which provides
   a helm repository including the charts from the previous step.
4. Package everything from the above steps into an ISO image.  The ISO
   image is built using [Elemental Toolkit](https://github.com/rancher/elemental-toolkit/),
   and is based on [harvester/os2](https://github.com/harvester/os2),
   which in turn is based on [SLE Micro](https://www.suse.com/products/micro/).

The built ISO image is written to the `dist/artifacts` directory.

## Harvester Installation Process

Harvester can be installed by either [booting the Harvester ISO](https://docs.harvesterhci.io/v1.2/install/index/),
or via [PXE Boot](https://docs.harvesterhci.io/v1.2/install/pxe-boot-install).
When booting via ISO, `harvester-installer` runs interactively on the
system console to allow you to configure the system.  When booting via
PXE, you don't get the interactive installer - instead you need to
provide YAML files specifying the configuration to apply.

In both cases (ISO boot and PXE boot), the `harvester-installer` binary
still _runs_ in order to provision the system.  This is put in place by
[system/oem/91_installer.yaml](https://github.com/harvester/harvester-installer/blob/master/package/harvester-os/files/system/oem/91_installer.yaml)
which in turn calls [setup-installer.sh](https://github.com/harvester/harvester-installer/blob/master/package/harvester-os/files/usr/bin/setup-installer.sh)
to start the installer on tty1.

When booted via ISO, the installer will prompt for configuration
information (create a new cluster / join an existing cluster, what
disks to use, network config, etc.).  When booted via PXE, the kernel
command line parameter `harvester.install.automatic=true` causes the
interactive part to be skipped, and config will be retrieved from the
URL specified by `harvester.install.config_url`.

Either way (ISO or PXE), the installer writes the final config out to
a temporary file which is passed to [harv-install](https://github.com/harvester/harvester-installer/blob/master/package/harvester-os/files/usr/sbin/harv-install)
which in turn calls `elemental install` to provision the system.
The harv-install script also preloads all the container images.
Finally the system is rebooted.

On the newly installed system, `harvester-installer` remains active
on the console in order to show the cluster management URL along with
the current node's hostname and IP address.

## License
Copyright (c) 2024 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
