#!/bin/bash -e

if [ -z "$TTY" ]; then
    export TTY=$(tty)
fi

export TERM=linux

harvester-installer
# Do not allow bash prompt if the installer doesn't exit with status 0
bash -l
