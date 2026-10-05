import { useQuery } from '@tanstack/react-query';
import { type Href } from 'expo-router';
import { Image } from 'expo-image';
import { useEffect, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { api, img, unwrap, type Item, type ListItem, type MediaType, type Progress } from '../api/client';
import { useLayout } from '../theme';
import { Focusable } from './Focusable';
import { Button, Hero, icons, itemCard, progressCard, RankCard, Row, Spinner, styles as ui } from './ui';
import { t } from '../i18n';

export function useLibrary() {
  const list = useQuery({ queryKey: ['library', 'list'], queryFn: () => unwrap(api.GET('/library/list')) });
  const progress = useQuery({ queryKey: ['library', 'progress'], queryFn: () => unwrap(api.GET('/library/progress')) });
  return { list: list.data ?? [], progress: progress.data ?? [] };
}

/** Home, Shows and Movies: hero + "continue watching" + "my list" + TMDB rows. */
export function Browse({ type }: { type?: MediaType }) {
  const { gutter } = useLayout();
  const rows = useQuery({
    queryKey: ['home', type ?? ''],
    queryFn: () => unwrap(api.GET('/catalog/home', { params: { query: { type } } })),
  });
  const { list, progress } = useLibrary();

  if (rows.isPending) return <Spinner full />;
  if (rows.isError) return <Text style={[ui.error, { padding: gutter, paddingTop: 80 }]}>{rows.error.message}</Text>;

  // Trending titles that can fill the banner: an image and a synopsis.
  const heroes = (rows.data[0]?.items ?? []).filter((i) => i.backdrop && i.overview).slice(0, HERO_COUNT);
  const keep = (t: string) => !type || t === type;
  const resume = progress.filter((p) => keep(p.type));
  const mine = list.filter((i) => keep(i.type));

  return (
    <ScrollView style={ui.page}>
      {heroes.length > 0 && <HeroCarousel items={heroes} />}
      <View style={{ marginTop: -80, paddingBottom: 60 }}>
        {resume.length > 0 && (
          <Row<Progress> title={t('browse.continueWatching')} data={resume} keyOf={(p) => `${p.type}-${p.id}`} render={progressCard} />
        )}
        {mine.length > 0 && (
          <Row<ListItem> title={t('common.myList')} data={mine} keyOf={(i) => `${i.type}-${i.id}`} render={(i) => itemCard({ ...i, backdrop: '' })} />
        )}
        {rows.data.map((r) => (
          <Row<Item>
            key={r.title}
            title={r.title}
            data={r.items}
            keyOf={(i) => `${i.type}-${i.id}`}
            render={r.ranked ? (i, width, index) => <RankCard item={i} rank={index + 1} width={width} /> : itemCard}
          />
        ))}
      </View>
    </ScrollView>
  );
}

const HERO_COUNT = 8;
const HERO_INTERVAL = 8000;

/**
 * Netflix style banner cycling through trending titles. It pauses while the pointer is over it or one of its
 * buttons has the focus, so a TV user never sees the title change under the remote.
 */
function HeroCarousel({ items }: { items: Item[] }) {
  const { gutter } = useLayout();
  const [index, setIndex] = useState(0);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const paused = hovered || focused;
  const item = items[index % items.length];

  useEffect(() => {
    if (paused || items.length < 2) return;
    const timer = setTimeout(() => setIndex((i) => (i + 1) % items.length), HERO_INTERVAL);
    return () => clearTimeout(timer);
  }, [index, paused, items.length]);

  // Fetch the next backdrop ahead so the crossfade never shows an empty frame.
  useEffect(() => {
    const next = img(items[(index + 1) % items.length]?.backdrop, 'w1280');
    if (next) Image.prefetch(next);
  }, [index, items]);

  const focusProps = { onFocus: () => setFocused(true), onBlur: () => setFocused(false) };

  return (
    <View onPointerEnter={() => setHovered(true)} onPointerLeave={() => setHovered(false)}>
      <Hero
        item={item}
        overlay={
          items.length > 1 && (
            <View style={[styles.dots, { right: gutter, bottom: 110 }]}>
              {items.map((it, i) => (
                <Focusable
                  key={it.id}
                  onPress={() => setIndex(i)}
                  {...focusProps}
                  accessibilityLabel={it.title}
                  style={(active) => [styles.dot, i === index % items.length && styles.dotCurrent, active && styles.dotActive]}
                >
                  {null}
                </Focusable>
              ))}
            </View>
          )
        }
      >
        <View style={ui.heroActions}>
          <Button
            icon={icons.play}
            label={t('common.play')}
            href={`/watch/${item.type}/${item.id}${item.type === 'tv' ? '?s=1&e=1' : ''}` as Href}
            hasTVPreferredFocus
            {...focusProps}
          />
          <Button kind="grey" icon={icons.info} label={t('common.moreInfo')} href={`/title/${item.type}/${item.id}` as Href} {...focusProps} />
        </View>
      </Hero>
    </View>
  );
}

export function Page({ children }: { children: React.ReactNode }) {
  const { gutter } = useLayout();
  return (
    <ScrollView style={ui.page} contentContainerStyle={{ padding: gutter, paddingTop: 48, paddingBottom: 60 }} keyboardShouldPersistTaps="handled">
      {children}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  dots: { position: 'absolute', flexDirection: 'row', gap: 6, zIndex: 2 },
  dot: { width: 18, height: 4, borderRadius: 2, backgroundColor: 'rgba(255,255,255,0.35)' },
  dotCurrent: { width: 32, backgroundColor: '#fff' },
  dotActive: { transform: [{ scaleY: 1.8 }], backgroundColor: '#fff' },
});
