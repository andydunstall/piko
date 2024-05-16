#!/bin/bash

set -e

# The version must be supplied from the environment. Do not include the
# leading "v".
if [ -z $VERSION ]; then
    echo "Please specify a version."
    exit 1
fi

git tag -a -m "Version $VERSION" "v${VERSION}" main

exit 0
