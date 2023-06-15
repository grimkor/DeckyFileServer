#!/usr/bin/env bash
CLI_LOCATION="$(pwd)/cli"
echo "Building plugin in $(pwd)"
printf "Please input sudo password to proceed.\n"

# read -s sudopass

# printf "\n"
NODE_ENV=production cd ui && npm run build && cd ..
cp -r ui/dist/* defaults/web

echo $sudopass | sudo $CLI_LOCATION/decky plugin build $(pwd)

# unzip
#unzip -o out/DeckyFileServer.zip -d out
## add ui
#NODE_ENV=production cd ui && npm run build && cd ..
#mkdir -p out/DeckyFileServer/ui
#cp -r ui/dist out/DeckyFileServer/ui/
### zip
#rm out/DeckyFileServer.zip
#cd out
#zip -r DeckyFileServer.zip DeckyFileServer
