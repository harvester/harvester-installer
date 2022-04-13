#!/bin/bash -x

# make sure we run as root
if [ ! $(id -u) -eq 0 ]; then
    echo "$(basename "${0}"): must be run as root" >&2
    exit 1
fi

pschildren() {
    ps -e -o ppid= -o pid= | \
    sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
    grep -w "^$1" | \
    cut -f2
}

pstree() {
    for pid in "$@"; do
        echo ${pid}
        for child in $(pschildren ${pid}); do
            pstree ${child}
        done
    done
}

killtree_term() {
    kill $(
        { set +x; } 2>/dev/null;
        pstree "$@";
        set -x;
    ) 2>/dev/null
}

killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        pstree "$@";
        set -x;
    ) 2>/dev/null
}

getshims() {
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w 'rke2/data/[^/]*/bin/containerd-shim' | cut -f1
}

systemctl stop rke2-server.service || true
systemctl stop rke2-agent.service || true

killtree_term $({ set +x; } 2>/dev/null; getshims; set -x)
sleep 15
killtree $({ set +x; } 2>/dev/null; getshims; set -x)
