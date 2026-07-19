# XP2P Connection Guide

This guide is organized from simple to complex. Start with a single A-B tunnel,
then layer in redirects, DNS handling, and multi-node chains.

## Start here

- Install xp2p: [Install](getting-started/install.md)
- Create your first tunnel: [First Tunnel (A-B)](guides/single-tunnel.md)
- Add policy routing and name-based rules: [Redirects Within A-B](guides/redirects.md)
- Build multi-node chains: [Chain (C2-B-A-C1)](guides/chain.md)
- Explore variants (multi-clients, split/full tunnel, DNS): [Advanced variants](guides/advanced.md)

## How xp2p works (flows)

- Deploy handshake (what it changes, what it does not): [Deploy flow](flows/deploy-flow.md)
- Desired/Live serialization and request generations: [Commit protocol](flows/desired-live-commit-protocol.md)
- Desired → Live apply mechanism: [Apply flow](flows/apply-flow.md)
- How Desired inputs become `xray.json`: [Config compilation](flows/config-compilation.md)
- How runtime status is derived: [Tunnel status logic](flows/tunnel-status.md)
- HA control plane and client failover: [High Availability](flows/high-availability.md)

## Concepts

- Terms used throughout the docs: [Terminology](getting-started/terminology.md)

## Lab scope

- OpenWrt hosts for A and B.
- Alpine guests for C1 and C2 (used in the chain scenario).
- Commands use the default paths and config dirs.

## Conventions

- A = server node, B = client node.
- C1, C2 = downstream guests behind NAT on B and A.
- Replace example IPs, users, and passwords with your actual values.
