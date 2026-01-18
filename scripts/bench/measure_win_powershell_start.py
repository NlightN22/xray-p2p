from __future__ import annotations

import argparse
import base64
import statistics
from pathlib import Path

import sys

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT))

from tests.host import common  # noqa: E402


def measure(host, samples: int) -> list[float]:
    ps_script = "\n".join(
        [
            "$sw = [System.Diagnostics.Stopwatch]::StartNew()",
            "powershell -NoProfile -NonInteractive -NoLogo -Command 1 | Out-Null",
            "$sw.Stop()",
            "$sw.Elapsed.TotalMilliseconds",
        ]
    )
    encoded = base64.b64encode(ps_script.encode("utf-16le")).decode("ascii")
    cmd = f"powershell -NoProfile -NonInteractive -NoLogo -EncodedCommand {encoded}"
    values: list[float] = []
    for _ in range(samples):
        result = host.run(cmd)
        if result.rc != 0:
            raise RuntimeError(
                "PowerShell timing command failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        raw = (result.stdout or "").strip().splitlines()
        values.append(float(raw[-1]))
    return values


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Measure guest PowerShell startup time over SSH."
    )
    parser.add_argument(
        "--vagrant-dir",
        default="infra/vagrant/windows10",
        help="Path to Vagrant environment directory.",
    )
    parser.add_argument(
        "--machine",
        default="win10-a",
        help="Vagrant machine name (e.g. win10-a).",
    )
    parser.add_argument(
        "--samples",
        type=int,
        default=5,
        help="Number of samples to collect.",
    )
    args = parser.parse_args()

    vagrant_dir = Path(args.vagrant_dir).resolve()
    host = common.get_ssh_host(vagrant_dir, args.machine)
    samples = max(1, int(args.samples))
    values = measure(host, samples)

    avg = statistics.mean(values)
    sorted_vals = sorted(values)
    p95_index = max(0, int((0.95 * len(sorted_vals)) + 0.5) - 1)
    p95 = sorted_vals[p95_index]

    print(f"machine={args.machine}")
    print(f"samples_ms={','.join(f'{v:.2f}' for v in values)}")
    print(f"avg_ms={avg:.2f}")
    print(f"p95_ms={p95:.2f}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
