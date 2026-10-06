// app.json, plus the version given at build time (APP_VERSION=1.2.3, a release tag without its "v"): the update
// check compares it to the latest release, and Android needs versionCode to grow for an update to install.
module.exports = ({ config }) => {
  const version = (process.env.APP_VERSION || config.version).replace(/^v/, '');
  const [major = 0, minor = 0, patch = 0] = version.split('.').map(Number);
  return {
    ...config,
    version,
    android: { ...config.android, versionCode: major * 10000 + minor * 100 + patch },
    plugins: [...config.plugins, './plugins/release-signing'],
  };
};
