[![Support me on Ko-fi](https://img.shields.io/badge/Ko--fi-Support%20Me-ff5e5b?style=flat-square&logo=ko-fi&logoColor=white)](https://ko-fi.com/grimbakor)
# DeckyFileServer

## Overview

DeckyFileServer is a plugin that allows you to quickly browse to files on your steam deck from a browser on another device.

## How to Install

1. [Install Decky Plugin on your Steam Deck](https://github.com/SteamDeckHomebrew/decky-loader).
2. Use the Decky store to install this plugin.

## How to use?

WARNING: Before using this plugin please be aware that this plugin does expose your selected folder and sub-folders to the network. Do not use this plugin on untrusted networks.

1. Use the settings page to set the folder you wish to browse externally.
2. Click the "Start Server" button.
3. Optional: Change the port from the default 8000 if the address is said to be in use.
4. Browse to the address shown on the panel on any device connected to the same network. You will be shown a security warning at this point, this is because the plugin is using a self-signed certificate. You can safely ignore this warning but follow browser-specific instructions on how to do so.
5. Click folders to browse into them, click on files to download them.

NOTE: The plugin will disable the server if it hasn't been used for 1 minute, this is to help prevent leaving your file system exposed by mistake. Pending downloads will continue to progress even after this timeout has started.

## How to build

1. Clone this repo.
2. Install pnpm globally ie. `npm install --global pnpm`
3. Run `pnpm i`
4. Run `pnpm run build`
6. Back in the project root run `.vscode/build.sh`. This will build the UI and the plugin into the `out/` folder.

# License
This is licensed under GNU GPLv3.
