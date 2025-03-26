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
		bridge vlan add vid 2-4094 dev $INTERFACE self
		bridge vlan add vid 2-4094 dev {{ .Bond }}

		{{ if ne .Role "" -}}
		iptables -P INPUT DROP

		iptables -A INPUT -p tcp --dport 80 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
		iptables -A INPUT -p tcp --dport 443 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
		iptables -A INPUT -p tcp --dport 22 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
		iptables -A INPUT -p udp --dport 8472 -j ACCEPT
		iptables -A INPUT -p tcp -m multiport --dports 6443:6444 -j ACCEPT
		iptables -A INPUT -p tcp -m multiport --dports 10248:10250 -j ACCEPT
		iptables -A INPUT -p tcp --dport 10010 -j ACCEPT
		iptables -A INPUT -p tcp --dport 9091 -j ACCEPT
		iptables -A INPUT -p tcp --dport 9099 -j ACCEPT
		{{ if or (eq .Role "default") (eq .Role "management") -}}
		iptables -A INPUT -p tcp --dport 9345 -j ACCEPT
		iptables -A INPUT -p tcp -m multiport --dports 10256:10260 -j ACCEPT
		iptables -A INPUT -p tcp -m multiport --dports 2379:2382 -j ACCEPT
		iptables -A INPUT -p tcp -m multiport --dports 2399:2402 -j ACCEPT

		iptables -A INPUT -p tcp --dport 2112 -j ACCEPT
		{{ else -}}
		iptables -A INPUT -p tcp --dport 10256 -j ACCEPT
		{{ end -}}
		iptables -A INPUT -m conntrack --ctstate ESTABLISHED -j ACCEPT
		{{ end -}}
		;;
esac
