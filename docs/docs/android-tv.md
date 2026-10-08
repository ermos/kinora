---
sidebar_position: 4
title: Android TV
---

kinora has a native Android TV app (Google TV included), built from the same code as the web app. It is designed for
the remote: everything is reachable with the arrows, OK and Back.

![kinora on an Android TV](/img/screenshots/android-tv.webp)

## Install the app

1. Get the APK: download `kinora-android-tv.apk` from the
   [latest GitHub release](https://github.com/ermos/kinora/releases/latest), or [build it yourself](#build-the-apk).
2. Copy it to the TV and install it: with `adb install kinora-android-tv.apk`, a USB stick, or the **Downloader** app on the
   TV. You may have to allow installs from unknown sources.
3. On first launch, enter your server address, as seen from the TV: `http://192.168.1.10:8080`. It is saved on the
   device and can be changed from the sign-in screen.

Plain HTTP is allowed, so a server on your local network works without TLS.

## Sign in

Typing a password with a remote is painful, so the TV shows a short code instead. On a computer or phone already
signed in to kinora, open `http://<your-server>:8080/link` and enter the code. The TV signs in to that account.

## Updates

Once signed in, the app asks the server for the latest version (`GET /api/v1/update`) and, if it is newer, offers to
install it, with the release notes, **Later** and **Skip this version**. Where the server looks depends on `APP_ENV`:

- `production` (default): the latest GitHub release, cached for a day. An admin can force a check from the Account
  page (**Check for updates**).
- `development`: the APK built locally with `npm run tv:apk`, served through a link signed for one hour. Run the
  server from the root of the repository (`make run`).

## Build the APK

### Locally

You need the Android SDK and JDK 17 (see [Test on an emulator](#test-on-an-emulator) for the setup on a Mac).

```bash
cd ui
npm ci
npm run tv:apk
```

The APK is written to `ui/android/app/build/outputs/apk/release/app-release.apk`. Without a release key, it is signed
with the debug key.

### In the cloud, without the SDK

```bash
cd ui
npx eas-cli@latest build -p android --profile tv
```

This needs a free Expo account (`eas init` the first time). The APK link is printed at the end.

### Release builds

These variables are read when building the APK:

| Variable | Description |
|---|---|
| `APP_VERSION` | APK version (`1.2.3`, a `v1.2.3` tag works too). The `versionCode` is derived from it, so each release must increase it. |
| `KINORA_KEYSTORE` | Path to the release keystore. |
| `KINORA_KEYSTORE_PASSWORD` | Keystore password. |
| `KINORA_KEY_ALIAS` | Key alias. |

Android only installs an update signed with the same key as the installed app: keep the keystore safe, outside the
repository. Create one with:

```bash
keytool -genkeypair -keystore release.keystore -alias kinora -keyalg RSA -keysize 4096 -validity 10000
```

Then build with the variables from `.env`:

```bash
cd ui && set -a && . ../.env && set +a
APP_VERSION=1.0.1 npm run tv:apk
```

On each release, the Release workflow builds the APK and attaches it to the GitHub release. It signs it with the
repository secrets `KINORA_KEYSTORE_BASE64` (`base64 -i release.keystore`), `KINORA_KEYSTORE_PASSWORD` and
`KINORA_KEY_ALIAS`, or with the debug key when they are missing.

## Test on an emulator

On a Mac:

1. Install [Android Studio](https://developer.android.com/studio). In **SDK Manager** (**Show Package Details**),
   install an **Android TV** or **Google TV** system image, **ARM 64 v8a** (API 31 or later) on Apple Silicon.
2. **Device Manager > Create device**, category **TV**, **Television (1080p)**, with that image. In
   **Advanced Settings**, 4 GB of internal storage is enough. Start it before step 4.
3. Install JDK 17, the one React Native recommends (`brew install openjdk@17`). With Java 24 and later, including the
   JDK bundled with Android Studio, the CMake configuration of the Android plugin fails on a JVM warning. Then point
   the terminal to the SDK and that JDK:

   ```bash
   export ANDROID_HOME=~/Library/Android/sdk
   export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
   export PATH=$ANDROID_HOME/platform-tools:$PATH
   ```

4. `cd ui && npm run tv:android`: TV prebuild, build and install on the emulator, with hot reload.
5. In the app, the server address seen from the emulator is `http://10.0.2.2:8080` (the Mac's `localhost`).

Drive the emulator's remote with the keyboard arrows, Enter to select and Escape to go back.
