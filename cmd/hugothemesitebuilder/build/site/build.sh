#!/bin/bash

function try() {
    "$@"
    code=$?
    if [ $code -ne 0 ]; then
        echo "$1 failed: exit status $code"
        exit 1
    fi
}

# Silent pushd
pushd() {
    command pushd "$@" >/dev/null
}

# Silent popd
popd() {
    command popd "$@" >/dev/null
}

pushd "../.."
try go run main.go build
popd
if [ -z  ${NETLIFY} ] || [ "$CONTEXT" == "production" ]
then
    echo "Build for production"
    try hugo --gc --minify
else
    echo "Build for ${DEPLOY_PRIME_URL}"
    try hugo --gc --minify -b $DEPLOY_PRIME_URL
fi
