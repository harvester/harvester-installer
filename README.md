harvester-installer
========
[![Build Status](https://github.com/harvester/harvester-installer/actions/workflows/build.yml/badge.svg)](https://github.com/harvester/harvester-installer/actions/workflows/build.yml)

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

During the build, the harvester source and addons will be pulled
automatically from https://github.com/harvester/harvester.git and
https://github.com/harvester/addons.git resectively.  If you would
like to use an exiting local copy of either or both repositories
instead, for example to pick up some development work in progress,
you can do so as follows:

```sh
 $ export LOCAL_HARVESTER_SRC=/path/to/local/harvester/repo
 $ export LOCAL_ADDONS_SRC=/path/to/local/addons/repo
 $ make
```

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

The installer will run some preflight checks to ensure the system
meets minimum hardware requirements.  If any of these checks
fail when run interactively, the first page of the installer will
indicate which checks failed, and give you the option to proceed or
not.  When installing via PXE, if any checks fail, installation will
abort and the failed checks will be visible on the system console,
and also logged to /var/log/console.log in the installation environment.
If you wish to bypass the preflight checks for testing purposes during
automated installation, set the `harvester.install.skipchecks=true`
kernel command line parmaeter.

Either way (ISO or PXE), the installer writes the final config out to
a temporary file which is passed to [harv-install](https://github.com/harvester/harvester-installer/blob/master/package/harvester-os/files/usr/sbin/harv-install)
which in turn calls `elemental install` to provision the system.
The harv-install script also preloads all the container images.
Finally, the system is rebooted.

On the newly installed system, `harvester-installer` remains active
on the console in order to show the cluster management URL along with
the current node's hostname and IP address.

## Hacking the Interactive Part of `harvester-installer`

Ordinarily `harvester-installer` needs to be run from a booted ISO,
so it can actually install a system.  But, if you're only working
on changes to the interactive part of the installer (e.g. adding
or changing fields, or altering the workflow) and don't actually
need to perform final installation, the binary can be quickly tested
using vagrant with vagrant-libvirt.  Here's how:

```sh
 $ USE_LOCAL_IMAGES=true make build
 $ vagrant up
 $ vagrant ssh
 > sudo DEBUG=true TTY=/dev/tty /vagrant/harvester-installer
```

Be sure the terminal window you use is fairly tall, or it will bail
out with "panic: invalid dimensions" after you get past the networking
screen.  To break out of the installer, hit CTRL-C.  If you rebuild
the binary it can be sync'd back to a running vagrant box with
`vagrant rsync`.

To remote debug the `harvester-installer` in your IDE, you need to
install the `delve` package in your Vagrant box first.

```sh
 $ zypper addrepo https://download.opensuse.org/repositories/devel:/languages:/go/SLE_15_SP4/devel:languages:go.repo
 $ zypper refresh
 $ zypper install delve
```

Then rebuild the `harvester-installer` app and start it in the
Vagrant box.

```sh
 $ REMOTE_DEBUG=true USE_LOCAL_IMAGES=true make build
 $ vagrant rsync
 $ vagrant ssh
 > sudo DEBUG=true TTY=/dev/tty dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec /vagrant/harvester-installer
```

The `harvester-installer` app will now listen for remote connections
on port `2345`.

Run `vagrant ssh-config` on your host to get the IP address that is 
needed to connect the IDE debugger to the `harvester-installer` app.

If you are using the Goland IDE, check out their [documentation](https://www.jetbrains.com/help/go/go-remote.html)
for information on how to attach to the `harvester-installer` process
on the remote Vagrant box.

If you are using VS Code, create a remote debug configuration in
`.vscode/launch.json` then press F5 to start debugging:

```json
{
	"name": "Remote",
	"type": "go",
	"request": "attach",
	"mode": "remote",
	"port": 2345,
	"host": "IP_ADDRESS_OF_VAGRANT_BOX",
}
```

## License
Copyright (c) 2025 [SUSE, LLC.](https://www.suse.com/)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
