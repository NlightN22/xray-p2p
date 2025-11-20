#!/bin/sh
set -eu

opkg update
opkg install coreutils-nohup coreutils-timeout >/dev/null 2>&1 || opkg install coreutils-nohup coreutils-timeout
