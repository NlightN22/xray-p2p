#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "Usage: copy_file.sh <source> <dest>" >&2
  exit 2
fi

src=$1
dst=$2

if [ ! -f "$src" ]; then
  echo "source file not found: $src" >&2
  exit 3
fi

mkdir -p "$(dirname "$dst")"
cp "$src" "$dst"
