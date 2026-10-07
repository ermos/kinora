---
sidebar_position: 5
title: Sources and quality
---

## Link order

When you press play, every link is resolved in parallel (10 seconds at most) and its HLS playlist is read to measure
the real resolution (4K, 1080p, 720p, SD). Links are then sorted:

1. by audio language, following the instance language. In French: French audio first (an unspecified language counts
   as French), then VOSTFR.
2. within a language, best quality first.

Dead links (hoster or CDN unreachable) go last. The result is kept for 10 minutes, and the stream already resolved is
used directly for playback. If a stream fails, the player moves on to the next link.

## French sources (vStream)

Every movie, TV show and anime source that vStream keeps active and that still answers is ported.

| Source | Content | Notes |
|---|---|---|
| Movix | movies, TV shows | |
| Purstream | movies, TV shows, anime | JSON API, direct multi-audio HLS |
| Coflix | movies, TV shows | search through the WordPress REST API |
| Wiflix | movies, TV shows | domain followed through its "new address" page |
| French Stream | movies, TV shows | VFF, VFQ and VOSTFR links |
| Cpasmieux | movies, TV shows | |
| Kepliz | movies | direct HLS |
| Anime-Sama | anime | VOSTFR and VF |
| French Anime | anime | VOSTFR and VF |
| Streaming Integrale | movies, TV shows, anime | not from vStream. Needs FlareSolverr |

## English sources (Scrubs V2)

Scrubs V2 is the English counterpart of vStream: an Exodus fork that scrapes free streaming sites.

| Source | Content | Notes |
|---|---|---|
| Levidia | movies, TV shows | `go.php` links redirected to Lulustream |
| Bstsrs | TV shows | Needs FlareSolverr |
| Project Free TV | movies, TV shows | Needs FlareSolverr |

## Hosters

Supported: Lulustream, Vidzy, Uqload, Voe (and its clones, detected by content), Vidmoly, Veev, Filemoon, Mixdrop,
Streamtape, Sendvid, Sibnet, Lecteur3, Abyss, the JW Player family (Filelions, Streamhide, Streamwish, Savefiles...)
and any unknown embed link that uses the same player.

Not supported: Dood and hgcloud (JavaScript challenge), Netu/Waaw (captcha).

## A source looks broken?

Sites change often. `make test-live` queries the real sources (one movie, one episode) and resolves every link. It is
the first thing to run when a source seems broken. See [Development](./development.md#port-a-source-or-a-hoster) to
fix or add one.
