#!/bin/bash
set -e

if [ "$K3OS_INSTALL_POWER_OFF" = true ] || grep -q 'k3os.install.power_off=true' /proc/cmdline; then
    poweroff -f
else
    echo " * Installation completed"
    echo " * Rebooting system in 5 seconds"
    sleep 5
    reboot -f
fi
