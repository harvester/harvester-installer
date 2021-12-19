#!/bin/bash
# Create a systemd drop-in unit to run installer as TTY Dashboard on a
# "non-system-console" virtual console or user specified serial console.

create_drop_in()
{
  DROP_IN_DIRECTORY=$1

  echo "Create installer drop-in in ${DROP_IN_DIRECTORY}..."
  mkdir -p ${DROP_IN_DIRECTORY}
  cp /etc/tty-dashboard-override.conf "${DROP_IN_DIRECTORY}/override.conf"
}

# reverse the ttys to start from the last one
for TTY in $(cat /sys/class/tty/console/active); do
  tty_num=${TTY#tty}

  # If console is on tty1 ~ tty64, we will show Harvester TTY Dashboard on other virtual terminal
  if [[ $tty_num =~ ^[0-9]+$ ]]; then
    unset dashboard_tty
    if [[ $tty_num -ge 6 ]]; then
        dashboard_tty="tty2"
    else
        dashboard_tty="tty$((tty_num+1))"
    fi
    create_drop_in "/run/systemd/system/getty@${dashboard_tty}.service.d"
    systemctl enable "getty@${dashboard_tty}.service"
    systemctl disable getty@tty1.service
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
