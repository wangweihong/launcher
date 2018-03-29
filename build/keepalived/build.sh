#!/bin/bash

cd `dirname $0`

VERSION="v1.0.3"

docker build -t 192.168.18.250:5002/launcher/keepalived:${VERSION} .
