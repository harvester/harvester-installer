#!/bin/bash
# Create a systemd drop-in unit to run installer on the first console tty. e.g.,
# if a system is booted with `console=tty1 console=ttyS0` parameters, the 
# script replaces the default login prompt on tty1 wth the installer.


create_drop_in()
{
  DROP_IN_DIRECTORY=$1

  echo "Create installer drop-in in ${DROP_IN_DIRECTORY}..."
  mkdir -p ${DROP_IN_DIRECTORY} 
  cat > "${DROP_IN_DIRECTORY}/override.conf" <<"EOF"
[Service]
EnvironmentFile=-/etc/rancher/installer/env

# Do not show kernel messages on this TTY, it messes up the installer UI
# NOTE: it doesn't work for serial console
ExecStartPre=/usr/bin/setterm --msg off

# Disable systemd messages before starting the installer
# See https://www.freedesktop.org/software/systemd/man/systemd.html#SIGRTMIN+21
# Send SIGRTMIN+21
ExecStartPre=/usr/bin/kill -s 55 1

# Enable systemd messager after stopping the installer
# Send SIGRTMIN+20
ExecStopPost=/usr/bin/kill -s 54 1

# clear the original command in getty@.service
ExecStart=

# override with the new command
ExecStart=-/sbin/agetty -n -l /usr/bin/start-installer.sh %I $TERM
EOF

}


# reverse the ttys to start from the last one
for TTY in $(cat /sys/class/tty/console/active); do
  tty_num=${TTY#tty}

  # tty1 ~ tty64
  if [[ $tty_num =~ ^[0-9]+$ ]]; then
    create_drop_in "/run/systemd/system/getty@${TTY}.service.d"
    break
  fi

  # might be serial console

  # check type is not 0
  tty_type=$(cat "/sys/class/tty/${TTY}/type")
  if [ "x${tty_type}" = "x0" ]; then
    continue
  fi
  
  create_drop_in "/run/systemd/system/serial-getty@${TTY}.service.d"
  break
done


systemctl daemon-reload
