# XP2P Connection Guide

This guide is organized from simple to complex. Start with a single A-B tunnel,
then layer in redirects, DNS handling, and multi-node chains.

## Scope

- OpenWrt hosts for A and B.
- Alpine guests for C1 and C2 (used in the chain scenario).
- Commands use the default paths and config dirs.

## Conventions

- A = server node, B = client node.
- C1, C2 = downstream Alpine guests behind B and A.
- Replace example IPs, users, and passwords with your actual values.

## Documents

- 01-single-tunnel.md
- 02-redirects.md
- 03-chain.md
- 04-advanced.md
- 05-tunnel-status.md
- 06-apply-flow.md
- 07-deploy-flow.md
- 08-config-compilation.md
