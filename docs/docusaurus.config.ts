import { themes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'kinora',
  tagline: 'Every movie. Every show. Your server.',
  favicon: 'img/favicon.png',
  url: 'https://kinora.stream',
  baseUrl: '/',
  organizationName: 'ermos',
  projectName: 'kinora',
  onBrokenLinks: 'throw',
  i18n: { defaultLocale: 'en', locales: ['en'] },
  stylesheets: ['https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,100..900&display=swap'],
  presets: [
    [
      'classic',
      {
        docs: { editUrl: 'https://github.com/ermos/kinora/tree/main/docs/' },
        blog: false,
        theme: { customCss: './src/css/custom.css' },
      } satisfies Preset.Options,
    ],
  ],
  themeConfig: {
    // The app only has a dark theme, so does its site.
    colorMode: { defaultMode: 'dark', disableSwitch: true, respectPrefersColorScheme: false },
    navbar: {
      title: 'KINORA',
      items: [
        { type: 'doc', docId: 'intro', label: 'Docs', position: 'left' },
        { href: 'https://github.com/ermos/kinora', label: 'GitHub', position: 'right' },
      ],
    },
    footer: {
      copyright: 'kinora does not host any content. Check what your local law allows before you stream.',
    },
    prism: { theme: themes.vsDark, darkTheme: themes.vsDark, additionalLanguages: ['bash', 'ini'] },
  } satisfies Preset.ThemeConfig,
};

export default config;
