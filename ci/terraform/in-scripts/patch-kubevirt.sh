#!/bin/bash -ex

wait_kubevirt() {
  # Wait for kubevirt to be deployed
  namespace=$1
  name=$2

  echo "Waiting for KubeVirt to be deployed..."
  while [ true ]; do
    kubevirt=$(kubectl get kubevirts.kubevirt.io $name -n $namespace -o yaml)

    current_phase=$(echo "$kubevirt" | yq e '.status.phase' -)
    if [ "$current_phase" = "Deployed" ]; then
      echo "KubeVirt is deployed"
      break
    fi

    echo "KubeVirt current phase: $current_phase"
    sleep 5
  done
}

MANIFEST=$(mktemp --suffix=.yml)
trap "rm -f $MANIFEST" EXIT

cat >$MANIFEST<<EOF
spec:
  configuration:
    developerConfiguration:
      useEmulation: true
EOF

kubectl patch kubevirts.kubevirt.io/kubevirt -n harvester-system --patch-file $MANIFEST --type merge
wait_kubevirt harvester-system kubevirt
