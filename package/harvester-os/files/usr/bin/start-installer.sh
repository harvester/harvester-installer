#!/bin/bash -e

if [ -z "$TTY" ]; then
    export TTY=$(tty)
fi

export TERM=linux

tty_num=${TTY#/dev/tty}
if [[ ${tty_num} =~ ^[0-9]+$ ]]; then
  # Switch virtual terminal
  chvt ${tty_num}
fi

harvester-installer
# Do not allow bash prompt if the installer doesn't exit with status 0
bash -l
