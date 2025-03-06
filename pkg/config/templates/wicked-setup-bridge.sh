#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
	pre-up)
		# enable vlan-aware
		ip link set $INTERFACE type bridge vlan_filtering 1
		;;

	post-up)
		# accept all vlan, PVID=1 by default
		bridge vlan add vid {{ .VlanID }} dev $INTERFACE self
		bridge vlan add vid {{ .VlanID }} dev {{ .IntfName }}
		;;
esac
