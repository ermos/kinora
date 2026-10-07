# kinora

Streaming multi-source auto-hébergé, interface façon Netflix. Le moteur de sources est un portage Go d'addons Kodi,
sans Kodi : [vStream](https://github.com/Kodi-vStream/venom-xbmc-addons) pour le français,
[Scrubs V2](https://github.com/jewbmx/repo) pour l'anglais.

- **Catalogue** : TMDB (tendances, genres, fiches, saisons, épisodes).
- **Sources** : quand on lance un titre, toutes les sources actives sont interrogées en parallèle pour ce film ou cet
  épisode. Les liens sont triés (VF/MULTI d'abord) et essayés dans l'ordre, avec repli automatique.
- **Hébergeurs** : chaque lien d'embed (vidzy, uqload, lulustream...) est résolu en flux HLS/MP4.
- **Proxy** : le flux passe par le serveur (Referer/User-Agent imposés par les CDN, pas de CORS). Les URLs sont
  signées en HMAC et expirent après 12 h : ce n'est pas un proxy ouvert.
- **Passer l'intro / le générique** : pour les animés, les boutons « Passer l'intro » et « Passer le générique »
  (ou « Épisode suivant ») s'appuient sur les timestamps communautaires d'[AniSkip](https://aniskip.com), via la
  correspondance TMDB vers MyAnimeList de [Fribb/anime-lists](https://github.com/Fribb/anime-lists). Désactivable par
  profil.
- **Comptes et profils** : comptes créés par l'admin, jusqu'à 5 profils par compte. Chaque profil a sa liste et son
  historique de lecture ("Reprendre la lecture").
- **Cloudflare** : les sites protégés par un challenge Cloudflare ne sont interrogés que si un admin renseigne un
  serveur [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) (page Compte, pris en compte sans redémarrage).
  Une requête challengée passe par FlareSolverr, puis le cookie `cf_clearance` obtenu est réutilisé en direct pour ce
  domaine jusqu'au challenge suivant. Le `docker-compose.yml` fournit un service `flaresolverr` prêt à l'emploi.
- **Domaines** : les URLs des sites sont synchronisées automatiquement toutes les 6 h depuis un `sites.json` par
  langue : celui de vStream pour le français (mis à jour par leur équipe plusieurs fois par semaine), et
  `internal/scraper/en/sites.json` de ce dépôt pour l'anglais (Scrubs garde ses domaines dans le code). Rien à
  configurer.

Un seul binaire Go (UI Expo exportée pour le web et embarquée) et une base PostgreSQL.

Documentation (en anglais) : https://ermos.github.io/kinora/, sources dans `docs/`.

## Démarrer

```sh
cp .env.example .env   # renseigner TMDB_API_KEY (clé gratuite sur themoviedb.org)
docker compose up -d   # kinora + PostgreSQL, http://localhost:8080
```

Les données vivent dans PostgreSQL (volume `postgres`) ; sauvegarde :
`docker compose exec postgres pg_dump -U kinora kinora > kinora.sql`.

Au premier lancement, la page `/setup` crée le compte administrateur.

Une instance qui tournait sur SQLite (avant PostgreSQL) se migre une fois, base PostgreSQL vide :
`docker compose run --rm -v ./data:/data kinora import-sqlite /data/kinora.db` (ou `kinora import-sqlite data/kinora.db`).
Utilisateurs, sessions, profils, liste, progression et réglages sont copiés.

En local sans Docker :

```sh
docker compose up -d postgres   # ou n'importe quel PostgreSQL, via DATABASE_URL
make build-web && TMDB_API_KEY=... DATABASE_URL=postgres://kinora:kinora@localhost:5432/kinora?sslmode=disable ./bin/kinora
# en dev, avec rechargement à chaud : `make dev` (Expo sur :8081) + `make run-dev`, puis http://localhost:8080
```

## Architecture

```
cmd/kinora            point d'entrée, wiring, synchro sites.json
internal/scraper       moteur : hébergeurs (hosters.go), unpacker JS, synchro des domaines
  fr/                  sources françaises (portage vStream)
  en/                  sources anglaises (portage Scrubs V2) et leur sites.json
internal/stream        proxy HLS/fichiers, signature HMAC
internal/tmdb          client TMDB avec cache mémoire
internal/api           API REST /api/v1 (annotations swaggo -> OpenAPI 3.1)
internal/store, db     PostgreSQL (pgx), migrations SQL embarquées, import d'une base SQLite
ui/                    Expo (React Native) : web et Android TV, tvOS ensuite
  src/app/             routes expo-router
  src/components/      Focusable, Sidebar, Player.web.tsx (hls.js), Player.tsx (expo-video)
  src/api/             client typé généré depuis le spec (openapi-typescript)
```

`make openapi` régénère le spec depuis les annotations Go puis le client TypeScript. `make openapi-check` échoue si le
spec committé est périmé.

## Langues

La langue est un réglage de l'instance, choisi au setup parmi les langues gérées de bout en bout, et modifiable ensuite
par un admin (page Compte). Elle fixe la langue
du catalogue TMDB, de l'interface, les sources interrogées (chaque source déclare son public dans `Langs`) et l'ordre
des pistes (pour le français : VF, puis VOSTFR). Langues gérées : français (sources vStream) et anglais (sources
Scrubs V2). Chaque langue est un package de `internal/scraper` qui enregistre sa langue et ses sources.

Ajouter une langue :

1. `internal/scraper/<code>/` : un package qui ajoute sa `Language` (code, nom, locale TMDB, ordre des pistes) et ses
   sources, importé dans `cmd/kinora/main.go`.
2. `internal/api/i18n.go` : les titres des rangées de l'accueil (un test vérifie qu'il n'en manque aucun).
3. `internal/api/auth.go` : le code dans les tags `enums`, puis `make openapi`.
4. `ui/src/i18n/<code>.ts` : le dictionnaire de l'interface, déclaré dans `ui/src/i18n/index.ts`. Le build TypeScript
   échoue tant qu'il manque.
5. Au moins une source qui déclare cette langue : sans elle, la langue n'est pas proposée au setup.

Les erreurs de l'API renvoient un `code` stable (`invalid_credentials`...) en plus d'un message anglais. L'interface
traduit le code : chaque code doit figurer dans la section `errors` de chaque dictionnaire, sinon le build TypeScript
échoue (et un test Go vérifie que chaque code est déclaré).

## UI : pensée pour la TV

L'UI est une app Expo, construite pour le web et Android TV. Tous les choix visent les apps TV :

- **Focus d'abord** : tout élément interactif passe par `Focusable`, qui gère focus (télécommande, clavier) et survol
  de la même façon. Entrée et Espace activent tout, la sidebar s'ouvre au focus comme sur l'app TV Netflix.
- **Pas d'éléments web-only** : pas de `<select>` ni de `confirm()`, remplacés par des pastilles (`Chip`) et des
  confirmations en deux appuis.
- **Modules supportés sur TV** : expo-router, expo-image, expo-video, react-native-svg, expo-linear-gradient,
  expo-secure-store.
- **Lecteur par plateforme** : `Player.web.tsx` (hls.js chargé à la demande) et `Player.tsx` (expo-video, HLS natif
  ExoPlayer/AVPlayer). Les deux sont pilotés par les mêmes contrôles façon Netflix (`PlayerControls`) : barre de
  progression avec temps restant, lecture/pause, ±10 s, son et plein écran sur le web, raccourcis clavier (Espace,
  flèches, F, M), tous focusables à la télécommande.
- **Proxy sans cookie** : l'URL de flux est signée et expire (12 h), elle suffit à elle seule. Les lecteurs natifs,
  AirPlay et Chromecast n'envoient pas les cookies de l'app.

### Android TV

L'app utilise le fork TV de React Native (`react-native-tvos`, même version que celui d'Expo) et le plugin
`@react-native-tvos/config-tv` : avec `EXPO_TV=1`, le prebuild génère une app Android TV (lanceur leanback, bannière
`assets/tv-banner.png`, icône `assets/tv-icon.png`). Le HTTP en clair est autorisé pour joindre une instance du réseau
local. Au premier lancement, l'app demande l'adresse du serveur (`http://192.168.1.10:8080`), gardée sur l'appareil et
modifiable depuis l'écran de connexion.

Construire l'APK :

- **En local** (SDK Android et JDK 17, voir ci-dessous) : `cd ui && npm run tv:apk`. L'APK sort dans
  `ui/android/app/build/outputs/apk/release/app-release.apk` : il s'installe tel quel (`adb install`, une clé USB, ou
  l'app Downloader sur la TV). Sans clé de release, il est signé avec la clé de debug.
- **Dans le cloud, sans SDK** : `npx eas-cli@latest build -p android --profile tv` (compte Expo gratuit, `eas init`
  la première fois). Le lien de l'APK s'affiche à la fin.

Mises à jour depuis l'app : au lancement, une fois connectée, l'app Android demande la dernière release au serveur
(`GET /api/v1/update`, `{version, notes, apk}`) et, si elle est plus récente, propose de l'installer (nouveautés,
« Plus tard », « Ignorer cette version »). Selon `APP_ENV` :

- `production` (défaut) : la dernière release GitHub (`tag_name`, `body`, l'asset `.apk`), gardée un jour. Un admin
  peut forcer la vérification depuis la page Compte (« Vérifier les mises à jour »).
- `development` : l'APK construit en local (`npm run tv:apk`), version lue dans
  `ui/android/app/build/outputs/apk/release/output-metadata.json`, servi par `GET /api/v1/update/apk` via un lien
  signé valable une heure (route absente en production). Lancer le serveur depuis la racine du dépôt (`make run`).

Variables lues au build de l'APK :

- `APP_VERSION` : version de l'APK (`1.2.3`, un tag `v1.2.3` marche aussi), `versionCode` en découle. Chaque
  release doit l'augmenter.
- `KINORA_KEYSTORE`, `KINORA_KEYSTORE_PASSWORD`, `KINORA_KEY_ALIAS` : clé de release. Android n'installe une mise à
  jour que signée avec la même clé que l'app en place : la garder précieusement, hors du dépôt. Une clé se crée avec
  `keytool -genkeypair -keystore release.keystore -alias kinora -keyalg RSA -keysize 4096 -validity 10000`.

```sh
cd ui && set -a && . ../.env && set +a
APP_VERSION=1.0.1 npm run tv:apk
```

Tester sur un Mac :

1. Installer [Android Studio](https://developer.android.com/studio), puis dans *SDK Manager* (*Show Package
   Details*) une image système **Android TV** ou **Google TV** en **ARM 64 v8a** (API 31 ou plus) sur un Mac Apple
   Silicon.
2. *Device Manager* > *Create device* > catégorie **TV** > *Television (1080p)*, avec l'image installée ; dans
   *Advanced Settings*, 4 Go de stockage interne suffisent (12 Go par défaut). Le démarrer avant l'étape 4.
3. Installer le JDK 17, celui que React Native recommande (`brew install openjdk@17`) : avec Java 24 et plus, dont
   le JDK livré avec Android Studio, la configuration CMake du plugin Android échoue sur un avertissement de la JVM.
   Puis pointer le terminal vers le SDK et ce JDK :
   `export ANDROID_HOME=~/Library/Android/sdk JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home`
   et `export PATH=$ANDROID_HOME/platform-tools:$PATH`.
4. `cd ui && npm run tv:android` : prebuild TV, compilation et installation sur l'émulateur, avec rechargement à chaud.
5. Dans l'app, l'adresse du serveur vue depuis l'émulateur est `http://10.0.2.2:8080` (le `localhost` du Mac).

La télécommande de l'émulateur se pilote avec les flèches du clavier, Entrée pour valider et Échap pour revenir.

## Porter une source ou un hébergeur

Les sites changent souvent : `make test-live` interroge les vraies sources (un film, un épisode) et résout chaque lien.
C'est le premier réflexe quand une source semble cassée.

**Source** : un fichier dans le package de sa langue, `internal/scraper/fr/<id>.go` ou `internal/scraper/en/<id>.go`.
L'id est la clé du site dans le `sites.json` de la langue (celui de vStream, ou `en/sites.json` pour l'anglais, dont
les clés sont les noms de fichiers des scrapers Scrubs).
Il n'y a pas de menus à porter : seule compte la fonction `Find`, qui reçoit le titre (localisé et original), l'année,
l'ID TMDB et éventuellement saison/épisode, et renvoie des liens. Le code Python de vStream donne les URLs et les regex.
`FindAll`/`Find` appliquent le même nettoyage que `cParser` de vStream, les regex se copient donc presque telles quelles
(Go utilise RE2 : pas de backreferences ni de lookahead).

**Hébergeur** : une entrée dans `internal/scraper/hosters.go`, avec les noms d'hôtes repris de `checkHoster` dans
`resources/lib/gui/hoster.py`, et une fonction `Resolve` qui renvoie l'URL du flux et les headers attendus par le CDN.
`Unpack` gère les scripts `eval(function(p,a,c,k,e,d)...)`.

## État du portage

### Français (vStream)

Toutes les sources films, séries et animés que vStream maintient actives et qui répondent encore sont portées.
Chacune est vérifiée en conditions réelles par `make test-live`.

| Source | Contenu | Remarques |
|---|---|---|
| Movix | films, séries | |
| Purstream | films, séries, animés | API JSON, HLS direct multi-audio |
| Coflix | films, séries | recherche via l'API REST WordPress (celle de vStream est cassée côté site) |
| Wiflix | films, séries | domaine suivi via sa page « nouvelle adresse » |
| French Stream | films, séries | liens VFF, VFQ et VOSTFR |
| Cpasmieux | films, séries | |
| Kepliz | films | HLS direct |
| Anime-Sama | animés | VOSTFR et VF |
| French Anime | animés | VOSTFR et VF |
| Streaming Integrale | films, séries, animés | hors vStream. Cloudflare : demande FlareSolverr. Lecteurs Lecteur3 et Abyss (strp2p répond « vidéo supprimée » partout) |

Non portées : TV en direct et sport (pas un catalogue), téléchargement direct (extreme_down, wawacity... : demandent
un débrideur), sources que vStream a lui-même désactivées ou retirées (souvent derrière un challenge Cloudflare),
sites morts, et adkami / anime-ultime / otaku-attitude (licences redirigées vers Crunchyroll, recherche cassée,
catalogue de téléchargement). Les sites de streaming que vStream marque Cloudflare (cpasmal, dulourd, juststream)
demandent en plus un captcha Turnstile pour chaque lien, que FlareSolverr ne résout pas ; french_stream_lol est le même
site que French Stream.

### Anglais (Scrubs V2)

Scrubs V2 est l'équivalent anglophone de vStream : un fork d'Exodus qui scrape les sites de streaming gratuits.
`en/sites.json` reprend ses 58 scrapers « working » au format de vStream, avec `active` selon un test réel
(octobre 2026) : 17 répondent encore.

| Source | Contenu | Remarques |
|---|---|---|
| Levidia | films, séries | liens `go.php` redirigés vers Lulustream |
| Bstsrs | séries | Cloudflare : demande FlareSolverr |
| Project Free TV | films, séries | Cloudflare : demande FlareSolverr. Watchseries (watchseries.cyou) partage la même base de liens |

Écartées après test : series9movies (lecteurs morts ou non gérés), m4ufree et tvids (uniquement des lecteurs maison type vidsrc / 2embed), PrimeWire / PrimeSrc (chaque lien demande un
captcha Turnstile), Goojara (lecteur Wootly maison), et les clones sflix / bflix (lecteurs chiffrés).

Hébergeurs : Lulustream, Vidzy, Uqload, Voe (et ses clones, détectés au contenu), Vidmoly, Veev, Filemoon, Mixdrop,
Streamtape, Sendvid, Sibnet, Lecteur3, Abyss (MP4 découpé et en-tête chiffré, reconstitué par le proxy), la famille JW player (Filelions, Streamhide, Streamwish, Savefiles...) et tout lien
d'embed inconnu qui utilise le même lecteur. Non gérés : Dood et hgcloud (challenge JavaScript), Netu/Waaw (captcha).

## Qualité et ordre des liens

Quand on lance un titre, chaque lien est résolu en parallèle (10 s maximum) et la playlist HLS est lue pour mesurer
la vraie résolution (4K, 1080p, 720p, SD). Les liens sont ensuite triés : audio français d'abord (une langue non
précisée compte comme VF), puis VOSTFR, et dans chaque langue la meilleure qualité d'abord. Les liens morts (hébergeur
ou CDN injoignable) passent en dernier. Le résultat est gardé 10 minutes, et le flux déjà résolu sert directement à la
lecture.
