#!/bin/sh

find . -iname "*_test.go" | grep -v "./vendor" | xargs dirname | sort -u | xargs echo $(sed -s "s/\./github.com\/kubernetes-incubator\/cluster-capacity/") | xargs go test
