// Signs release APKs with the key in KINORA_KEYSTORE (password KINORA_KEYSTORE_PASSWORD, alias KINORA_KEY_ALIAS),
// read at build time: Android only installs an update signed with the same key as the installed app. Without it,
// release builds keep the debug key.
const { withAppBuildGradle } = require('expo/config-plugins');

const releaseConfig = `
        release {
            if (System.getenv('KINORA_KEYSTORE')) {
                storeFile file(System.getenv('KINORA_KEYSTORE'))
                storePassword System.getenv('KINORA_KEYSTORE_PASSWORD')
                keyAlias System.getenv('KINORA_KEY_ALIAS') ?: 'kinora'
                keyPassword System.getenv('KINORA_KEYSTORE_PASSWORD')
            }
        }`;

module.exports = (config) =>
  withAppBuildGradle(config, (c) => {
    let gradle = c.modResults.contents;
    if (!gradle.includes("System.getenv('KINORA_KEYSTORE')")) {
      gradle = gradle.replace(/signingConfigs \{\n/, (m) => m + releaseConfig.slice(1) + '\n');
      // The release build type is the second "signingConfig signingConfigs.debug", after the debug one.
      let seen = 0;
      gradle = gradle.replace(/signingConfig signingConfigs\.debug/g, (m) =>
        ++seen === 2 ? "signingConfig System.getenv('KINORA_KEYSTORE') ? signingConfigs.release : signingConfigs.debug" : m,
      );
      if (seen !== 2) throw new Error('release-signing: unexpected app/build.gradle layout');
    }
    c.modResults.contents = gradle;
    return c;
  });
