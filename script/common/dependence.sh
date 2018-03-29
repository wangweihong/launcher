#!/bin/bash

dependence::checkDeps(){
    Log::Register "${FUNCNAME}"

    if command -v docker > /dev/null 2>&1; then
        Log "OK, docker has been installed"
    else
        Log "Failed, please install docker first."
        Utils::exitCode 1
    fi

    Log "Check dependence software"
    for cmd in "socat"
    do
        command -v ${cmd} &> /dev/null
        [ $? -gt 0 ] && Log "command ${cmd} not found, please install first" && Utils::exitCode 1
    done

    Log::UnRegister "${FUNCNAME}"
}