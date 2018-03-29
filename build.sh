#!/bin/bash
SCRIPTHOME=$(cd `dirname $0`;pwd)

cd $SCRIPTHOME

build(){
	echo ${SCRIPTHOME}

	# clean
	rm -rf ${GOPATH}/src/ufleet/launcher
	mkdir -p ${GOPATH}/src/ufleet/launcher

	# copy source files
	cp -rf ./code ${GOPATH}/src/ufleet/launcher/

	# begin to build
	cd ${GOPATH}/src/ufleet/launcher/code
	GO15VENDOREXPERIMENT=1 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -installsuffix cgo -o kube-launcher

	# move binary file to dest dir
	mv kube-launcher ${SCRIPTHOME}/
}

main(){
	build
}

main
