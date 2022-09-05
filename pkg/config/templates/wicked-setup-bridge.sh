#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
    pre-up)
        # enable vlan-aware
        ip link set $INTERFACE type bridge vlan_filtering 1
        ;;

	post-up)
        # inherit MAC address from {{ . }}
        ip link set dev $INTERFACE address $(ip -json link show dev {{ . }} | jq -j '.[0]["address"]')

        # accept all vlan, PVID=1 by default
        bridge vlan add vid 2-4094 dev $INTERFACE self
        bridge vlan add vid 2-4094 dev {{ . }}
		;;
esac
