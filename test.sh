#!/bin/bash

set -euf -o pipefail

go test ./...
go vet ./...
echo "SUCCESS"
