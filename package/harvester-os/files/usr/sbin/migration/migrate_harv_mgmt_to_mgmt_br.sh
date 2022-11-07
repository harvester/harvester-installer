#!/bin/bash

HARV_CONFIG="${HARV_CONFIG:-/oem/99_custom.yaml}"
HARV_MGMT="harvester-mgmt"
IFCFG=
IFROUTE=

function detect_mgmt () {
	yq -e '.stages.initramfs[0].files[] | select(.path == "/etc/sysconfig/network/ifcfg-'${HARV_MGMT}'") | .content' "$HARV_CONFIG" > /dev/null
}

function migrate_mgmt_config () {
	MODE=
	IFCFG=$(yq '.stages.initramfs[0].files[] | select(.path == "/etc/sysconfig/network/ifcfg-'${HARV_MGMT}'") | .content' "$HARV_CONFIG")
	IFROUTE=$(yq '.stages.initramfs[0].files[] | select(.path == "/etc/sysconfig/network/ifroute-'${HARV_MGMT}'") | .content' "$HARV_CONFIG")
	printf "%s\n" "$IFCFG"
	printf "%s\n" "$IFROUTE"

	# check DHCP or static
	if printf "%s\n" "$IFCFG" | grep -q "BOOTPROTO='static'"; then
		MODE="static"
	elif printf "%s\n" "$IFCFG" | grep -q "BOOTPROTO='dhcp'"; then
		MODE="dhcp"
	else
		echo "error detect bootproto mode"
		exit 1
	fi

	# start patch
	# remove cluster network
	yq -i 'del( .stages.initramfs[0].files[] | select(.path == "*21-harvester-clusternetworks.yaml*"))' "$HARV_CONFIG"

	# remove all ifcfg and ifroute
	if [ $MODE == "dhcp" ]; then
		yq -i 'del( .stages.initramfs[0].commands[] | select(. == "rm -f /etc/sysconfig/network/ifroute-harvester-mgmt"))' "$HARV_CONFIG"
	fi
	yq -i 'del( .stages.initramfs[0].files[] | select(.path == "*ifcfg*"))' "$HARV_CONFIG"
	yq -i 'del( .stages.initramfs[0].files[] | select(.path == "*ifroute*"))' "$HARV_CONFIG"
	sed -i "s/iface: $HARV_MGMT/iface: \"\"/g" "$HARV_CONFIG"

	# add file
	if [ $MODE == "dhcp" ]; then
		yq -i '.stages.initramfs[0].commands += "rm -f /etc/sysconfig/network/ifroute-mgmt-br"' "$HARV_CONFIG"
	fi

	cat <<EOF | yq eval-all -i 'select(fileIndex==0).stages.initramfs[0].files += select(fileIndex==1) | select(fileIndex==0)' "$HARV_CONFIG" -
- path: /etc/wicked/scripts/setup_bond.sh
  permissions: 755
  owner: 0
  group: 0
  content: |+
    #!/bin/sh

    ACTION=\$1
    INTERFACE=\$2

    case \$ACTION in
    	post-up)
    		# inherit MAC address
    		ip link set dev mgmt-br address \$(ip -json link show dev \$INTERFACE | jq -j '.[0]["address"]')
    		# accept all vlan, PVID=1 by default
    		bridge vlan add vid 2-4094 dev \$INTERFACE
    		;;
    esac
  encoding: ""
  ownerstring: ""

- path: /etc/wicked/scripts/setup_bridge.sh
  permissions: 755
  owner: 0
  group: 0
  content: |+
    #!/bin/sh

    ACTION=\$1
    INTERFACE=\$2

    case \$ACTION in
    	pre-up)
    		# enable vlan-aware
    		ip link set \$INTERFACE type bridge vlan_filtering 1
    		;;

    	post-up)
    		# accept all vlan, PVID=1 by default
    		bridge vlan add vid 2-4094 dev \$INTERFACE self
    		bridge vlan add vid 2-4094 dev mgmt-bo
    		;;
    esac
  encoding: ""
  ownerstring: ""

- path: /etc/sysconfig/network/ifcfg-mgmt-br
  permissions: 384
  owner: 0
  group: 0
  content: |+
    STARTMODE='onboot'
    BOOTPROTO='${MODE}'
    BRIDGE='yes'
    BRIDGE_STP='off'
    BRIDGE_FORWARDDELAY='0'
    BRIDGE_PORTS='mgmt-bo'
    PRE_UP_SCRIPT="wicked:setup_bridge.sh"
    POST_UP_SCRIPT="wicked:setup_bridge.sh"

    $(printf "%s\n" "$IFCFG" | grep "IPADDR")
    $(printf "%s\n" "$IFCFG" | grep "NETMASK")
    $(printf "%s\n" "$IFCFG" | grep "DHCLIENT_SET_DEFAULT_ROUTE")
    $(printf "%s\n" "$IFCFG" | grep "MTU")
  encoding: ""
  ownerstring: ""

- path: /etc/sysconfig/network/ifcfg-mgmt-bo
  permissions: 384
  owner: 0
  group: 0
  content: |+
    STARTMODE='onboot'
    BONDING_MASTER='yes'
    BOOTPROTO='none'
    POST_UP_SCRIPT="wicked:setup_bond.sh"
$(printf "%s\n" "$IFCFG" | grep "BONDING_SLAVE_" | sed 's/^/    /')
    $(printf "%s\n" "$IFCFG" | grep "BONDING_MODULE_OPTS")

    $(printf "%s\n" "$IFCFG" | grep "MTU")
  encoding: ""
  ownerstring: ""
EOF

	if [ $MODE == "static" ]; then
		cat <<EOF | yq eval-all -i 'select(fileIndex==0).stages.initramfs[0].files += select(fileIndex==1) | select(fileIndex==0)' "$HARV_CONFIG" -
- path: /etc/sysconfig/network/ifroute-mgmt-br
  permissions: 384
  owner: 0
  group: 0
  content: |
    $(printf "%s\n" "$IFROUTE" | sed "s/$HARV_MGMT/mgmt-br/g")
  encoding: ""
  ownerstring: ""
EOF
	fi
}

if ! grep -q "$HARV_MGMT" "$HARV_CONFIG"; then
	echo "$HARV_MGMT not found. Skipping."
	exit 0
fi

if ! detect_mgmt; then
	echo "ifcfg-${HARV_MGMT} not found. Skipping."
	exit 0
fi

echo "$HARV_MGMT found."

# backup config with timestamp
TIMESTAMP=$(date "+%Y%m%d-%H%M%S")
cp "$HARV_CONFIG" "${HARV_CONFIG}.${TIMESTAMP}"

migrate_mgmt_config

