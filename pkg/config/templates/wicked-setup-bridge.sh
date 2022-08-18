#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
    pre-up)
        # inherit MAC address from bond-mgmt
        ip link set dev $INTERFACE address $(ip -json link show dev bond-mgmt | jq -j '.[0]["address"]')

        # enable vlan-aware
        ip link set $INTERFACE type bridge vlan_filtering 1

        # accept all vlan, PVID=1 by default
        bridge vlan add vid 2-4094 dev $INTERFACE self
        bridge vlan add vid 2-4094 dev bond-mgmt
        ;;
esac
