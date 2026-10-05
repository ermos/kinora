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

Un seul binaire Go (UI Expo exportée pour le web et embarquée, SQLite pur Go), aucune dépendance système.

## Démarrer

```sh
cp .env.example .env   # renseigner TMDB_API_KEY (clé gratuite sur themoviedb.org)
docker compose up -d   # http://localhost:8080
```

Au premier lancement, la page `/setup` crée le compte administrateur.

En local sans Docker :

```sh
make build-web && TMDB_API_KEY=... ./bin/kinora
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
internal/store, db     SQLite, migrations SQL embarquées
ui/                    Expo (React Native) : web aujourd'hui, Android TV / tvOS ensuite
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

L'UI est une app Expo. Seul le build web est produit pour l'instant, mais tous les choix visent les apps TV :

- **Focus d'abord** : tout élément interactif passe par `Focusable`, qui gère focus (télécommande, clavier) et survol
  de la même façon. Entrée et Espace activent tout, la sidebar s'ouvre au focus comme sur l'app TV Netflix.
- **Pas d'éléments web-only** : pas de `<select>` ni de `confirm()`, remplacés par des pastilles (`Chip`) et des
  confirmations en deux appuis.
- **Modules supportés sur TV** : expo-router, expo-image, expo-video, react-native-svg, expo-linear-gradient,
  expo-secure-store.
- **Lecteur par plateforme** : `Player.web.tsx` (hls.js chargé à la demande) et `Player.tsx` (expo-video, HLS natif
  ExoPlayer/AVPlayer, contrôles natifs pilotables à la télécommande).
- **Proxy sans cookie** : l'URL de flux est signée et expire (12 h), elle suffit à elle seule. Les lecteurs natifs,
  AirPlay et Chromecast n'envoient pas les cookies de l'app.

Pour activer les builds TV (non fait) :

1. `package.json` : `"react-native": "npm:react-native-tvos@0.86-stable"` (même version que le React Native d'Expo).
2. `npx expo install @react-native-tvos/config-tv -- --dev`, puis l'ajouter aux `plugins` de `app.json`.
3. `EXPO_TV=1 npx expo prebuild --clean`, puis `npx expo run:android` / `run:ios` sur un émulateur TV.
4. Reste à faire côté app : un écran « adresse du serveur » (aujourd'hui `EXPO_PUBLIC_API_URL`) et éprouver le
   lecteur natif sur appareil.

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
Streamtape, Sendvid, Sibnet, la famille JW player (Filelions, Streamhide, Streamwish, Savefiles...) et tout lien
d'embed inconnu qui utilise le même lecteur. Non gérés : Dood et hgcloud (challenge JavaScript), Netu/Waaw (captcha).

## Qualité et ordre des liens

Quand on lance un titre, chaque lien est résolu en parallèle (10 s maximum) et la playlist HLS est lue pour mesurer
la vraie résolution (4K, 1080p, 720p, SD). Les liens sont ensuite triés : audio français d'abord (une langue non
précisée compte comme VF), puis VOSTFR, et dans chaque langue la meilleure qualité d'abord. Les liens morts (hébergeur
ou CDN injoignable) passent en dernier. Le résultat est gardé 10 minutes, et le flux déjà résolu sert directement à la
lecture.
