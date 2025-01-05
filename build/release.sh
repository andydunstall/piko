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

if gh release edit $1 --verify-tag ;
then
  # Release exists, upload binaries
  gh release upload \
    $1 \
    bin/artifacts/*
else
  gh release create \
    $1 \
    bin/artifacts/* \
    --generate-notes
fi

