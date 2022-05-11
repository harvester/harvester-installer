#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
    pre-up)
        ip link set $INTERFACE type bridge vlan_filtering 1
        bridge vlan add vid 2-4094 dev $INTERFACE self
        bridge vlan add vid 2-4094 dev bond-mgmt
        ;;
esac
