## Testing the ISO image

This repo uses [luet-makeiso](https://github.com/Luet-lab/luet-makeiso) to produce a hybrid ISO image.
A hybrid ISO image should be able to boot on older computers that boot with legacy BIOS as well as 
newer computers that boot with UEFI BIOS.

To test that the build artefact is bootable, here are two test cases you can execute. Both test cases
depend on qemu and the UEFI test depends on the ovmf package, which is a port of the UEFI firmware to 
the qemu virtual machine.

## Legacy BIOS Boot Test

```
qemu-system-x86_64 -m 2048 \
  -cdrom ../dist/artifacts/harvester-master-amd64.iso
```

This test passes only if it reaches the point where you choose between "Create Harvester cluster" or "Join Harvester Cluster"

## UEFI BIOS Boot Test

```
qemu-system-x86_64 -m 2048 \
  -cdrom ../dist/artifacts/harvester-master-amd64.iso
  -bios ovmf-x86_64.bin
```

Note: On OpenSUSE, the `ovmf-x86_64.bin` file is in `/usr/share/qemu/`, on Ubuntu I had to use `find`.

This test passes only if it reaches the point where you choose between "Create Harvester cluster" or "Join Harvester Cluster"
