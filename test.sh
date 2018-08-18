#!/bin/bash

set -euf -o pipefail

go test ./...
go vet ./...
UNFORMATTED=$(go fmt ./...)
if [ ! -z "${UNFORMATTED}" ]; then
	echo "UNFORMATTED FILES:"
	echo ${UNFORMATTED}
	echo "FAILED"
	exit 1
fi
echo "SUCCESS"
