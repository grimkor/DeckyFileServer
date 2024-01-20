#!/usr/bin/env bash
CLI_LOCATION="$(pwd)/cli"
echo "Building plugin in $(pwd)"
#printf "Please input sudo password to proceed.\n"

# read -s sudopass

# build backend
cd backend &&  ./build.sh && cd ..


echo $sudopass | sudo $CLI_LOCATION/decky plugin build $(pwd)
cd backend && ./build.sh && cd ..

