#!/bin/sh -x

go test -v github.com/ingvagabund/cluster-capacity/pkg/client/emulator/strategy github.com/ingvagabund/cluster-capacity/pkg/client/emulator/restclient
