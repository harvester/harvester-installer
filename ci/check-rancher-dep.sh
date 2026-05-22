#!/bin/bash -e
# Verify that the Rancher charts listed in scripts/rancher/deps.yaml
# exist inside the rancher/rancher Docker image.
#
# Reads scripts/version-rancher and scripts/rancher/deps.yaml as the
# single source of truth — no arguments required.
#
# Checks performed for each chart (fleet, fleet-crd, rancher-webhook):
#   - Chart entry exists in the image's index.yaml
#   - Chart tarball (.tgz) exists at the expected path
#   - appVersion in index.yaml matches the value recorded in deps.yaml

TOP_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." &> /dev/null && pwd )"
SCRIPTS_DIR="${TOP_DIR}/scripts"

source "${SCRIPTS_DIR}/version-rancher"

DEPS_FILE="${SCRIPTS_DIR}/rancher/deps.yaml"

# Read chart and app versions from deps.yaml
FLEET_CHART_VERSION=$(yq e '.rancherDependencies.fleet.chart' "${DEPS_FILE}")
FLEET_APP_VERSION=$(yq e '.rancherDependencies.fleet.app' "${DEPS_FILE}")
FLEET_CRD_CHART_VERSION=$(yq e '.rancherDependencies.fleet-crd.chart' "${DEPS_FILE}")
FLEET_CRD_APP_VERSION=$(yq e '.rancherDependencies.fleet-crd.app' "${DEPS_FILE}")
WEBHOOK_CHART_VERSION=$(yq e '.rancherDependencies.rancher-webhook.chart' "${DEPS_FILE}")
WEBHOOK_APP_VERSION=$(yq e '.rancherDependencies.rancher-webhook.app' "${DEPS_FILE}")

RANCHER_IMAGE="rancher/rancher:${RANCHER_VERSION}"

echo "Checking Rancher charts for ${RANCHER_IMAGE}"
echo "  fleet:          chart=${FLEET_CHART_VERSION} app=${FLEET_APP_VERSION}"
echo "  fleet-crd:      chart=${FLEET_CRD_CHART_VERSION} app=${FLEET_CRD_APP_VERSION}"
echo "  rancher-webhook: chart=${WEBHOOK_CHART_VERSION} app=${WEBHOOK_APP_VERSION}"

# Global variables for cleanup
CONTAINER_ID=""
WORK_DIR=""
REPO_DIR=""

cleanup() {
  if [ -n "$CONTAINER_ID" ]; then
    echo "Cleaning up container ${CONTAINER_ID}..." >&2
    docker stop "$CONTAINER_ID" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_ID" >/dev/null 2>&1 || true
  fi
  if [ -n "$WORK_DIR" ] && [ -d "$WORK_DIR" ]; then
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

# Pull the Rancher image, extract the rancher-charts git repo via docker cp,
# and sparse-checkout index.yaml and the required chart asset directories.
# Sets globals: WORK_DIR, CONTAINER_ID, REPO_DIR
extract_rancher_charts() {
  echo "Pulling ${RANCHER_IMAGE}..."
  docker pull "${RANCHER_IMAGE}"

  readonly WORK_DIR=$(mktemp -d)

  docker create --cidfile="${WORK_DIR}/container_id" "${RANCHER_IMAGE}"
  readonly CONTAINER_ID=$(cat "${WORK_DIR}/container_id")

  docker cp "${CONTAINER_ID}:/var/lib/rancher-data/local-catalogs/v2/rancher-charts" "${WORK_DIR}/rancher-charts"
  echo "Extracted charts directory to ${WORK_DIR}/rancher-charts"

  repo_hash=$(ls "${WORK_DIR}/rancher-charts" | head -n1)
  echo "Charts repository hash: ${repo_hash}"
  readonly REPO_DIR="${WORK_DIR}/rancher-charts/${repo_hash}"

  # The repo is cloned with "--no-checkout" when being packaged
  # sparse checkout needed files
  pushd "${REPO_DIR}" >/dev/null
  git sparse-checkout set index.yaml assets/fleet assets/fleet-crd assets/rancher-webhook
  git checkout
  popd >/dev/null
}

# verify_chart chart_name chart_version expected_app_version
verify_chart() {
  local chart_name=$1
  local chart_version=$2
  local expected_app_version=$3
  local index_yaml="${REPO_DIR}/index.yaml"

  echo "Checking ${chart_name} chart=${chart_version} app=${expected_app_version}..."

  # Verify entry exists in index.yaml and app version matches
  local actual_app_version
  actual_app_version=$(yq e ".entries.${chart_name}[] | select(.version == \"${chart_version}\") | .appVersion" "${index_yaml}")

  if [ -z "$actual_app_version" ]; then
    echo "ERROR: ${chart_name} chart version ${chart_version} not found in index.yaml" >&2
    return 1
  fi

  if [ "$actual_app_version" != "$expected_app_version" ]; then
    echo "ERROR: ${chart_name} appVersion mismatch: expected=${expected_app_version} actual=${actual_app_version}" >&2
    return 1
  fi

  # Verify tarball exists
  local tarball_path="$REPO_DIR/assets/${chart_name}/${chart_name}-${chart_version}.tgz"
  if ! test -f "${tarball_path}"; then
    echo "ERROR: ${chart_name} tarball not found: ${tarball_path}" >&2
    return 1
  fi

  echo "PASS: ${chart_name} chart=${chart_version} app=${actual_app_version} tarball=OK"
}

extract_rancher_charts
verify_chart "fleet"           "${FLEET_CHART_VERSION}"   "${FLEET_APP_VERSION}"
verify_chart "fleet-crd"       "${FLEET_CRD_CHART_VERSION}" "${FLEET_CRD_APP_VERSION}"
verify_chart "rancher-webhook" "${WEBHOOK_CHART_VERSION}"  "${WEBHOOK_APP_VERSION}"

echo "PASS: All Rancher charts verified successfully in ${RANCHER_IMAGE}"
