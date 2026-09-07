# Overview

## What homa is

A peer-to-peer terminal chat with file transfer, written in Go.

Two machines exchange one address, then talk directly: encrypted, through NAT,
with no account, no coordination server, and no shared network. Neither side is
"the server". Both listen, both can call.

## Where it started

The project began from Tailscale's release of
[tailcat](https://github.com/tailscale/tailcat), which takes Tailscale's data
plane, WireGuard encryption plus NAT traversal plus DERP relays, and strips away
the control plane: no accounts, no tailnet, no admin panel. Reaching a peer needs
only a compact address string that encodes their public keys, a pre-shared key,
and which relay to meet at.

That removed the hard part of building a direct peer-to-peer chat. Prior art was
surveyed before starting:

- **Tox / toxic** is the closest existing thing, but runs its own DHT and has
  fought NAT traversal for years. homa borrows that layer instead of building it.
- **croc** and **magic-wormhole** have the best "just connect" experience,
  built on a short human-readable code rather than a long key. Worth stealing
  later; see the unplaced work in [roadmap.md](roadmap.md).
- **chat-tails** is a terminal chat over Tailscale, but requires everyone to be
  in the same tailnet, is centralized around one host, and has no file transfer.
  homa's whole point is that two people share nothing beforehand.

So the gap homa fills: terminal chat, plus file transfer, plus peer-to-peer with
no account, on top of infrastructure that already works.

The name is the Homa, the bird of Persian myth that never lands.
