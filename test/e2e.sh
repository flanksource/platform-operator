#!/bin/bash

set -ex

export TEST_E2E=true

k8s_version=1.11.0
# keeping older version around to reproduce any issue (just in case)
#k8s_version=1.10.1
goarch=amd64
goos="unknown"

if [[ "$OSTYPE" == "linux-gnu" ]]; then
  goos="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  goos="darwin"
fi

if [[ "$goos" == "unknown" ]]; then
  echo "OS '$OSTYPE' not supported. Aborting." >&2
  exit 1
fi

function header_text {
  echo "$header$*$reset"
}

# fetch k8s API gen tools and make it available under kb_root_dir/bin.
function fetch_kb_tools {
  if [ -n "$SKIP_FETCH_TOOLS" ]; then
    return 0
  fi

  header_text "fetching tools"
  kb_tools_archive_name="kubebuilder-tools-$k8s_version-$goos-$goarch.tar.gz"
  kb_tools_download_url="https://storage.googleapis.com/kubebuilder-tools/$kb_tools_archive_name"

  kb_tools_archive_path="$tmp_root/$kb_tools_archive_name"
  if [ ! -f $kb_tools_archive_path ]; then
    curl -sL ${kb_tools_download_url} -o "$kb_tools_archive_path"
  fi
  tar -zvxf "$kb_tools_archive_path" -C "$tmp_root/"
}

function setup_envs {
  header_text "setting up env vars"

  # Setup env vars
  export TEST_ASSET_KUBECTL=$kb_root_dir/bin/kubectl
  export TEST_ASSET_KUBE_APISERVER=$kb_root_dir/bin/kube-apiserver
  export TEST_ASSET_ETCD=$kb_root_dir/bin/etcd
}

tmp_root=/tmp
kb_root_dir=$tmp_root/kubebuilder

fetch_kb_tools

setup_envs

go test github.com/flanksource/platform-operator/test
