#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
	post-up)
		# inherit MAC address
		ip link set dev {{ . }} address $(ip -json link show dev $INTERFACE | jq -j '.[0]["address"]')

		# accept all vlan, PVID=1 by default
		bridge vlan add vid 2-4094 dev $INTERFACE
		;;

esac
