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

dst_dir=$(dirname "$dst")
if [ ! -d "$dst_dir" ]; then
  echo "Destination directory does not exist: $dst_dir" >&2
  exit 3
fi
cp "$src" "$dst"
