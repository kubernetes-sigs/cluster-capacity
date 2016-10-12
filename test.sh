#!/bin/sh

find . -iname "*_test.go" | grep -v "./vendor" | xargs dirname | sort -u | xargs echo $(sed -s "s/\./github.com\/ingvagabund\/cluster-capacity/") | xargs go test -v
