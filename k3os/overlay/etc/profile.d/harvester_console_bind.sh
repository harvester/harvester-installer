#!/bin/bash

# bind <F12>
bind -x '"\e[24~":"harvester-console"'

# dashboard mode
export HARVESTER_DASHBOARD=true
export TTY=$(tty)