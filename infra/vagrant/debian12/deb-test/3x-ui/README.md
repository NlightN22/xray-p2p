# Pinned 3x-ui compatibility fixture

This fixture runs 3x-ui `v2.8.11` from the immutable Linux amd64 manifest `sha256:ce050d75791a4576c0a5b2fdd207909efa7f88bf6a0a45c5424b949d5fd53432` on `deb-test-c`.

The test harness removes `state`, starts the Compose project, runs `setup.sh`, and owns cleanup. The administrative API is used only to create deterministic Trojan and VLESS inbounds. XP2P reads only `http://10.62.10.13:2096/sub/xp2pfixture2811` during this local fixture; production subscription URLs still require HTTPS.

The expected panel commit is `52fdf5d4296b4534e25d6221d82ec7d819a9b952`. The pinned source declares Xray module `v1.260206.0`, corresponding to Xray `v26.2.6`; the integration test must also verify the versions reported by the running binaries.

`mutate.sh` is a test-only helper for deterministic credential rotation,
transport security changes, disabling, enabling, and removing the Trojan offer
through the pinned panel administrative API.

## Advanced integration matrix

The ordinary Linux suite runs only the pinned-version contract and basic XP2P
Live traffic checks. Set `XP2P_RUN_EXTERNAL_SUBSCRIPTION_MATRIX=1` to include
the extended refresh, failure-injection, recovery, and isolation matrix.
