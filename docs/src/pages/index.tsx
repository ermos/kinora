import type { CSSProperties, ReactNode } from 'react';
import Head from '@docusaurus/Head';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import s from './index.module.css';

const GITHUB = 'https://github.com/ermos/kinora';

const icons = {
  home: 'M12 3 2 11h3v9h5v-6h4v6h5v-9h3z',
  docs: 'M5 3h10l4 4v14H5zM14 3v5h5M8 12h8M8 16h8',
  install: 'M12 3v12m-5-5 5 5 5-5M4 20h16',
  github:
    'M12 2a10 10 0 0 0-3.2 19.5c.5.1.7-.2.7-.5v-1.7c-2.8.6-3.4-1.3-3.4-1.3-.4-1.2-1.1-1.5-1.1-1.5-.9-.6.1-.6.1-.6 1 .1 1.6 1 1.6 1 .9 1.5 2.4 1.1 2.9.8.1-.6.4-1.1.6-1.3-2.2-.3-4.6-1.1-4.6-5a3.9 3.9 0 0 1 1-2.7c-.1-.3-.4-1.3.1-2.7 0 0 .8-.3 2.8 1a9.6 9.6 0 0 1 5 0c1.9-1.3 2.8-1 2.8-1 .5 1.4.2 2.4.1 2.7a3.9 3.9 0 0 1 1 2.7c0 3.9-2.3 4.7-4.6 5 .4.3.7.9.7 1.9V21c0 .3.2.6.7.5A10 10 0 0 0 12 2z',
};

function Icon({ d, fill }: { d: string; fill?: boolean }) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" aria-hidden="true" fill={fill ? 'currentColor' : 'none'} stroke={fill ? 'none' : 'currentColor'} strokeWidth="2" strokeLinejoin="round" strokeLinecap="round">
      <path d={d} />
    </svg>
  );
}

function Rail() {
  const docs = useBaseUrl('/docs/intro');
  const install = useBaseUrl('/docs/installation');
  const items = [
    { label: 'Home', href: '#top', d: icons.home, fill: true },
    { label: 'Docs', href: docs, d: icons.docs },
    { label: 'Install', href: install, d: icons.install },
    { label: 'GitHub', href: GITHUB, d: icons.github, fill: true },
  ];
  return (
    <nav className={s.rail} aria-label="Main">
      <a className={s.railLogo} href="#top" aria-label="kinora home">K</a>
      {items.map((i) => (
        <a key={i.label} className={s.railItem} href={i.href} aria-current={i.href === '#top' ? 'page' : undefined}>
          <Icon d={i.d} fill={i.fill} />
          <span>{i.label}</span>
        </a>
      ))}
    </nav>
  );
}

const features = [
  { tile: 'Feels familiar', title: 'The interface you already know', body: 'Rows, a big hero, profiles and "Continue watching". Movies, TV shows and anime from the TMDB catalog.', color: '#e50914' },
  { tile: 'All at once', title: 'Every source in parallel', body: 'Press play and kinora queries every active source, measures each stream, and starts the best one. Dead links fall back on their own.', color: '#2f80ed' },
  { tile: 'Remote first', title: 'Built for the couch', body: 'Web, phone and a native Android TV app. Everything works with a remote, a keyboard or a mouse.', color: '#27ae60' },
  { tile: 'Skip intro', title: 'Skip intro and credits', body: 'Community timestamps from AniSkip on anime, with a "Next episode" button. Off per profile if you prefer.', color: '#9b51e0' },
  { tile: '×5', title: 'Five profiles per account', body: 'Each one with its own list, likes and watch history. You create the accounts, nobody signs up.', color: '#f2780c' },
  { tile: 'One binary', title: 'One binary, one database', body: 'A Go server with the web app built in, next to PostgreSQL. A compose file and a free TMDB key are all it takes.', color: '#14a39a' },
  { tile: 'Signed streams', title: 'A private proxy', body: 'Streams go through your server with signed links that expire after 12 hours. Not an open proxy.', color: '#eb5ea8' },
];

const steps: { title: string; code: string; body: ReactNode }[] = [
  {
    title: 'Get the compose file',
    code: 'mkdir kinora && cd kinora\ncurl -fsSLO https://kinora.stream/docker-compose.yml',
    body: 'Docker is all you need. The kinora image runs on amd64 and arm64, a Raspberry Pi included.',
  },
  {
    title: 'Add your TMDB key',
    code: 'echo "TMDB_API_KEY=your_key" > .env',
    body: (
      <>
        Free on <a href="https://www.themoviedb.org/settings/api">themoviedb.org</a>. It powers the catalog and the artwork.
      </>
    ),
  },
  { title: 'Press play', code: 'docker compose up -d', body: 'Open port 8080 in your browser and create the admin account. That\'s it.' },
];

const RELEASES = `${GITHUB}/releases/latest`;

const clients = [
  { title: 'In your browser', body: 'Built into the server. Computers, tablets and phones, nothing to install.', d: 'M3 4h18v12H3zM8 20h8M12 16v4' },
  {
    title: 'On Android TV',
    body: 'A native app for Android TV and Google TV, made for the remote. Download the APK from the latest release, it then updates itself from your server.',
    d: 'M2 5h20v13H2zM7 22h10',
    link: { href: RELEASES, label: 'Download the APK' },
  },
];

export default function Home(): ReactNode {
  const install = useBaseUrl('/docs/installation');
  return (
    <div className={s.page} id="top">
      <Head>
        <title>kinora: every movie, every show, your server</title>
        <meta name="description" content="kinora is a self-hosted streaming server with a Netflix-style interface for the web and Android TV. One binary, one database, every source in parallel." />
        <meta name="theme-color" content="#141414" />
      </Head>
      <Rail />
      <main className={s.main}>
        <header className={s.hero}>
          <div className={s.backdrop} aria-hidden="true">K</div>
          <div className={s.heroContent}>
            <p className={s.wordmark}>KINORA</p>
            <h1 className={s.title}>Every movie. Every show. Your server.</h1>
            <p className={s.meta}>
              <span className={s.match}>100% yours</span>
              <span>Open source</span>
              <span className={s.badge}>4K</span>
              <span className={s.badge}>TV</span>
            </p>
            <p className={s.overview}>
              Your own streaming service, on your own server. Browse a catalog of every movie and show, press play, and kinora searches every
              source it knows to start the best stream it finds. In your browser, on your phone and on your Android TV.
            </p>
            <div className={s.actions}>
              <Link className={s.play} to={install}>
                <Icon d="M7 4v16l13-8z" fill />
                Install kinora
              </Link>
              <Link className={s.more} to={GITHUB}>
                <Icon d={icons.github} fill />
                View on GitHub
              </Link>
            </div>
          </div>
        </header>

        <section className={s.row} aria-labelledby="features">
          <h2 id="features" className={s.rowTitle}>Popular on your server</h2>
          <div className={s.scroller} tabIndex={0} aria-label="Features, scrollable">
            {features.map((f) => (
              <article key={f.title} className={s.card}>
                <div className={s.poster} style={{ '--tint': f.color } as CSSProperties} aria-hidden="true">
                  {f.tile}
                </div>
                <h3>{f.title}</h3>
                <p>{f.body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className={s.row} aria-labelledby="steps">
          <h2 id="steps" className={s.rowTitle}>Top 3 steps to get started</h2>
          <ol className={s.steps}>
            {steps.map((st, i) => (
              <li key={st.title} className={s.step}>
                <span className={s.rank} aria-hidden="true">{i + 1}</span>
                <div className={s.stepCard}>
                  <h3>{st.title}</h3>
                  <pre><code>{st.code}</code></pre>
                  <p>{st.body}</p>
                </div>
              </li>
            ))}
          </ol>
          <p className={s.next}>
            FlareSolverr, backups, reverse proxy and every setting: <Link to={install}>read the installation guide</Link>.
          </p>
        </section>

        <section className={s.row} aria-labelledby="clients">
          <h2 id="clients" className={s.rowTitle}>Watch on every screen</h2>
          <div className={s.clients}>
            {clients.map((c) => (
              <article key={c.title} className={s.stepCard}>
                <Icon d={c.d} />
                <h3>{c.title}</h3>
                <p>{c.body}</p>
                {c.link && (
                  <Link className={s.download} to={c.link.href}>
                    <Icon d={icons.install} />
                    {c.link.label}
                  </Link>
                )}
              </article>
            ))}
          </div>
        </section>

        <footer className={s.footer}>
          <span>Free and open source under the <a href={`${GITHUB}/blob/main/LICENSE`}>AGPL-3.0</a>. Made with Go, Expo and PostgreSQL.</span>
          <span>It does not host any content. Check what your local law allows before you stream.</span>
        </footer>
      </main>
    </div>
  );
}
