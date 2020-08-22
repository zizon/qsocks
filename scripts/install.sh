#!/bin/bash
current=`realpath .`

# downlaod go
go=`which go`
if [ -z $go ];then
    if [ ! -d go ];then
        curl -L https://golang.org/dl/go1.15.linux-amd64.tar.gz -o go.tar.gz
        tar -xf go.tar.gz
    fi

    # keep current location
    go="$current/go/bin/go"
fi

# build
cd ..
$go build

# may be overwrite
cp -f init/qsocks.service /etc/systemd/system
systemctl daemon-reload
systemctl stop qsocks

# copy new binary
if [ -d /usr/local/bin ];then
    mkdir -p /usr/local/bin
fi
cp -f qsocks /usr/local/bin

# restart
systemctl restart qsocks

# cleanup downlaod
cd $current
rm go.tar.gz