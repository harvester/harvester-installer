#!/bin/bash -e
# This script collects Rancher dependency information and generates a YAML file
# with chart and app versions for fleet, fleet-crd, and rancher-webhook.
#
# Usage:
#   $ collect-rancher-deps.sh [output-file]
#
# When output-file is omitted, defaults to scripts/rancher/deps.yaml.
#
# The script performs the following:
# 1. Fetches Rancher's build.yaml from GitHub to get expected chart versions
# 2. Derives the rancher/charts branch from the Rancher version
#    (e.g. v2.14.0 -> dev-v2.14)
# 3. Fetches each chart's Chart.yaml from rancher/charts to get appVersion
# 4. Writes the output file with chart and app version information
#
# Example output:
# ```
# rancherDependencies:
#   fleet:
#     chart: 109.0.0+up0.15.0
#     app: 0.15.0
#   fleet-crd:
#     chart: 109.0.0+up0.15.0
#     app: 0.15.0
#   rancher-webhook:
#     chart: 109.0.0+up0.10.0
#     app: 0.10.0
# ```

TOP_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." &> /dev/null && pwd )"
SCRIPTS_DIR="${TOP_DIR}/scripts"
DEPS_FILE="${SCRIPTS_DIR}/rancher/deps.yaml"

output_file="${1:-${DEPS_FILE}}"

source ${SCRIPTS_DIR}/version-rancher

# Derive rancher/charts branch from Rancher version (e.g. v2.14.0 -> dev-v2.14)
rancher_branch="dev-v$(echo "${RANCHER_VERSION}" | sed 's/^v//' | cut -d. -f1,2)"

# Fetch appVersion for a chart from rancher/charts GitHub repo
# Arguments: chart_name, chart_version, branch
# Returns: app_version (via stdout)
get_chart_app_version() {
  local chart_name=$1
  local chart_version=$2
  local branch=$3

  local url="https://raw.githubusercontent.com/rancher/charts/${branch}/charts/${chart_name}/${chart_version}/Chart.yaml"
  echo "Fetching ${chart_name} Chart.yaml from ${url}..." >&2

  local app_version
  app_version=$(curl -sf "$url" | yq e '.appVersion' -)

  if [ -z "$app_version" ] || [ "$app_version" = "null" ]; then
    echo "ERROR: appVersion not found for ${chart_name} ${chart_version} on branch ${branch}" >&2
    return 1
  fi

  echo "PASS: ${chart_name} ${chart_version} appVersion=${app_version}" >&2
  echo "${app_version}"
}

update_rancher_deps() {
  local rancher_version=$1
  local out_file=$2

  # Fetch rancher's build.yaml to get chart versions
  local rancher_build_yaml=$(mktemp)
  local build_yaml_url="https://raw.githubusercontent.com/rancher/rancher/${rancher_version}/build.yaml"
  echo "Fetching rancher build.yaml from ${build_yaml_url}"
  curl -sf "$build_yaml_url" -o "$rancher_build_yaml"

  if [ ! -s "$rancher_build_yaml" ]; then
    echo "Error: Failed to fetch build.yaml" >&2
    rm -f "$rancher_build_yaml"
    return 1
  fi

  local fleet_chart_version=$(yq e .fleetVersion "$rancher_build_yaml")
  local webhook_chart_version=$(yq e .webhookVersion "$rancher_build_yaml")

  rm -f "$rancher_build_yaml"

  echo "Fleet chart version: ${fleet_chart_version}"
  echo "Rancher webhook chart version: ${webhook_chart_version}"
  echo "rancher/charts branch: ${rancher_branch}"

  local fleet_app_version
  fleet_app_version=$(get_chart_app_version "fleet" "${fleet_chart_version}" "${rancher_branch}") || return 1

  local fleet_crd_app_version
  fleet_crd_app_version=$(get_chart_app_version "fleet-crd" "${fleet_chart_version}" "${rancher_branch}") || return 1

  local webhook_app_version
  webhook_app_version=$(get_chart_app_version "rancher-webhook" "${webhook_chart_version}" "${rancher_branch}") || return 1

  # Write output file
  mkdir -p "$(dirname "$out_file")"
  cat > "$out_file" <<EOF
rancherDependencies:
  fleet:
    chart: "${fleet_chart_version}"
    app: "${fleet_app_version}"
  fleet-crd:
    chart: "${fleet_chart_version}"
    app: "${fleet_crd_app_version}"
  rancher-webhook:
    chart: "${webhook_chart_version}"
    app: "${webhook_app_version}"
EOF
  echo "Written to ${out_file}"
}

echo "Collect Rancher dependencies for version ${RANCHER_VERSION} and output to ${output_file}..."
update_rancher_deps "$RANCHER_VERSION" "$output_file"
