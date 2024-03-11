#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
	pre-up)
		# enable vlan-aware
		ip link set $INTERFACE type bridge vlan_filtering 1
		# reset to default vlan state
		# accept all vlan, PVID=1 by default
		bridge vlan del vid 1-4094 dev $INTERFACE self
		# set the configured vlan id
		{{ if and (gt . 1) (le . 4095) -}}
		bridge vlan add vid {{ . }} dev $INTERFACE pvid untagged self
		{{ else -}}
		bridge vlan add vid 1 dev $INTERFACE pvid untagged self
		{{ end -}}
		;;

esac
