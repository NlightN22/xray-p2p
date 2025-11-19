#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-openwrt}"

template_dir_index() {
  local path=$1
  local title=$2
  shift 2
  local items=("$@")

  {
    echo '<!DOCTYPE html>'
    echo '<html><head><meta charset="utf-8"><title>'"$title"'</title></head><body>'
    echo '<h1>'"$title"'</h1>'
    echo '<ul>'
    for item in "${items[@]}"; do
      if [ -d "$path/$item" ]; then
        echo "  <li><a href=\"$item/\">$item/</a></li>"
      else
        echo "  <li><a href=\"$item\">$item</a></li>"
      fi
    done
    echo '</ul></body></html>'
  } > "$path/index.html"
}

generate_indexes() {
  local base=$1
  local level_title=$2

  mapfile -t entries < <(cd "$base" && ls -1)
  template_dir_index "$base" "$level_title" "${entries[@]}"

  for entry in "${entries[@]}"; do
    local full="$base/$entry"
    if [ -d "$full" ]; then
      if ls "$full"/*.ipk >/dev/null 2>&1; then
        mapfile -t files < <(cd "$full" && ls -1)
        template_dir_index "$full" "$level_title / $entry" "${files[@]}"
      else
        generate_indexes "$full" "$level_title / $entry"
      fi
    fi
  done
}

if [ ! -d "$ROOT" ]; then
  echo "ERROR: $ROOT directory not found" >&2
  exit 1
fi

generate_indexes "$ROOT" "xp2p OpenWrt feed"
