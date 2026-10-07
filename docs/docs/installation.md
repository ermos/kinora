---
sidebar_position: 2
title: Installation
---

kinora ships as a single Go binary (the web app is built in) next to a PostgreSQL database. Docker Compose runs both.

## Requirements

- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin.
- A free TMDB API key: create an account on [themoviedb.org](https://www.themoviedb.org/signup), then copy the
  **API Key** (v3) or the **API Read Access Token** (v4) from [settings/api](https://www.themoviedb.org/settings/api).
  Both work.

## Install with Docker Compose

```bash
git clone https://github.com/ermos/kinora
cd kinora
cp .env.example .env
```

Open `.env` and set your key:

```ini
TMDB_API_KEY=your_tmdb_key
```

Then start kinora:

```bash
docker compose up -d
```

This builds the image and starts three services:

| Service | Role |
|---|---|
| `kinora` | the server and the web app, on port `8080` |
| `postgres` | the database, stored in the `postgres` volume |
| `flaresolverr` | optional, lets kinora reach sites behind Cloudflare (see [Configuration](./configuration.md#flaresolverr)) |

## First launch

Open [http://localhost:8080](http://localhost:8080) (or `http://<server-ip>:8080` from another device). The first
visit redirects to `/setup`, where you:

1. create the **admin account**,
2. choose the **language** of the instance (French or English). It sets the catalog language, the interface, which
   sources are queried and the order of audio tracks. An admin can change it later from the Account page.

Other accounts are created by the admin from the Account page. There is no public sign-up.

## Update

```bash
git pull
docker compose up -d --build
```

Database migrations run on startup.

## Backup and restore

All data (users, profiles, lists, progress, settings) lives in PostgreSQL.

```bash
# backup
docker compose exec postgres pg_dump -U kinora kinora > kinora.sql

# restore into an empty database
docker compose exec -T postgres psql -U kinora kinora < kinora.sql
```

## Migrate from SQLite

Instances from before PostgreSQL stored their data in `data/kinora.db`. Import it once, into an empty PostgreSQL
database:

```bash
docker compose run --rm -v ./data:/data kinora import-sqlite /data/kinora.db
```

Users, sessions, profiles, lists, progress and settings are copied.

## Run without Docker

You need Go (see `go.mod` for the version), Node.js 22 and any PostgreSQL.

```bash
docker compose up -d postgres   # or your own PostgreSQL, through DATABASE_URL
make build-web                  # exports the web app and builds bin/kinora with it embedded
TMDB_API_KEY=your_tmdb_key \
DATABASE_URL='postgres://kinora:kinora@localhost:5432/kinora?sslmode=disable' \
./bin/kinora
```

## Put it behind a reverse proxy

kinora listens on plain HTTP. To reach it from outside your network, put it behind a reverse proxy that handles TLS
(Caddy, Traefik, nginx...). For example with Caddy:

```
kinora.example.com {
  reverse_proxy localhost:8080
}
```

Next: [connect your Android TV](./android-tv.md).
