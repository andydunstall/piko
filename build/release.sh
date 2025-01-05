#!/bin/bash

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "No release tag name given. Failing"
  exit 1
fi


if ! command -v gh 2>&1 >/dev/null
then
  echo "Github CLI could not be found"
  exit 2
fi

gh release create \
  $1 \
  bin/artifacts/* \
  --generate-notes
