#!/bin/bash

source /opt/harvester-mode

export TTY=$(tty)
export TERM="linux"

# Set a default window size for un-attached terminals like serial consoles
MIN_ROWS=24
MIN_COLS=80
read ROWS COLS <<< $(/usr/bin/stty -F ${TTY} size)
if [ $ROWS -lt $MIN_ROWS ] || [ $COLS -lt $MIN_COLS ]; then
    /usr/bin/stty -F "${TTY}" rows "${MIN_ROWS}" cols "${MIN_COLS}"
fi

harvester-console
/bin/bash --login
