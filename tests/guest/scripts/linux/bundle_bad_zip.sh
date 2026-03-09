#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: bundle_bad_zip.sh <output-archive>" >&2
  exit 2
fi

archive=$1

python3 - "$archive" <<'PY'
import sys
import zipfile

path = sys.argv[1]
with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("../evil.txt", "nope")
PY
