import { useQuery } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import { api, unwrap, useProfile } from '../../api/client';
import { Focusable } from '../../components/Focusable';
import { SearchKeyboard } from '../../components/SearchKeyboard';
import { Button, Field, Icon, icons, PosterGrid, Spinner, styles as ui } from '../../components/ui';
import { getItem, setItem } from '../../lib/storage';
import { t } from '../../i18n';
import { colors, useLayout } from '../../theme';

const HISTORY_MAX = 8;
const SUGGESTIONS_MAX = 6;
const RESULT_WIDTH = 130; // narrowest poster: the panel leaves the results less room than other grids

/**
 * Netflix layout: on the left the query (a remote-driven keyboard on TV), then suggestions while typing or the
 * recent searches; on the right the results, or what is trending before typing anything.
 */
export default function Search() {
  const params = useLocalSearchParams<{ q?: string }>();
  const q = params.q ?? '';
  const [input, setInput] = useState(q);
  const debounce = useRef<ReturnType<typeof setTimeout>>(undefined);
  const { width, rail, gutter, phone } = useLayout();
  const profile = useProfile();
  const history = useSearchHistory(profile?.id);

  const results = useQuery({
    queryKey: ['search', q],
    queryFn: () => unwrap(api.GET('/catalog/search', { params: { query: { q } } })),
    enabled: q !== '',
  });
  // Same query as the home page: usually already cached.
  const home = useQuery({ queryKey: ['home', ''], queryFn: () => unwrap(api.GET('/catalog/home', { params: { query: {} } })) });

  const type = (value: string) => {
    setInput(value);
    clearTimeout(debounce.current);
    debounce.current = setTimeout(() => router.setParams({ q: value.trim() || undefined }), 350);
  };
  const search = (value: string) => {
    clearTimeout(debounce.current);
    setInput(value);
    router.setParams({ q: value.trim() || undefined });
    history.add(value);
  };

  // Titles of the first results, the query itself left out.
  const suggestions = useMemo(() => {
    const seen = new Set([input.trim().toLowerCase()]);
    return (results.data ?? [])
      .map((r) => r.title)
      .filter((title) => !seen.has(title.toLowerCase()) && seen.add(title.toLowerCase()))
      .slice(0, SUGGESTIONS_MAX);
  }, [results.data, input]);

  // A share of the screen, so the keyboard and the results scale together.
  const panelWidth = Math.round(Math.min(420, Math.max(260, (width - rail) * 0.28)));
  const resultsWidth = phone ? width - 2 * gutter : width - rail - 3 * gutter - panelWidth;
  const trending = home.data?.[0]?.items ?? [];

  return (
    <ScrollView style={ui.page} contentContainerStyle={[styles.page, { padding: gutter }, phone && styles.pagePhone]} keyboardShouldPersistTaps="handled">
      <View style={[styles.panel, !phone && { width: panelWidth }]}>
        {Platform.isTV ? (
          <>
            <View style={styles.query}>
              <Icon d={icons.search} size={22} color={colors.muted} />
              <Text style={[styles.queryText, !input && { color: colors.muted }]} numberOfLines={1}>
                {input || t('browse.searchPlaceholder')}
              </Text>
            </View>
            <SearchKeyboard width={panelWidth} onKey={(k) => type(input + k)} onDelete={() => type(input.slice(0, -1))} />
          </>
        ) : (
          <Field
            value={input}
            onChangeText={type}
            onSubmitEditing={() => search(input)}
            placeholder={t('browse.searchPlaceholder')}
            autoFocus
            returnKeyType="search"
            accessibilityLabel={t('nav.search')}
            style={styles.field}
          />
        )}
        {input.trim() ? (
          suggestions.length > 0 && <Picks title={t('search.suggestions')} icon={icons.search} items={suggestions} onPick={search} />
        ) : (
          history.items.length > 0 && (
            <Picks
              title={t('search.recent')}
              icon={icons.history}
              items={history.items}
              onPick={search}
              action={<Button kind="outline" small label={t('search.clear')} onPress={history.clear} />}
            />
          )
        )}
      </View>

      <View style={styles.results}>
        {q ? (
          results.isPending ? (
            <Spinner />
          ) : results.data?.length ? (
            <PosterGrid items={results.data} width={resultsWidth} columnWidth={RESULT_WIDTH} onOpen={() => history.add(q)} />
          ) : (
            <Text style={ui.muted}>{t('browse.noResults', { q })}</Text>
          )
        ) : (
          trending.length > 0 && (
            <>
              <Text style={ui.h2}>{t('search.trending')}</Text>
              <PosterGrid items={trending} width={resultsWidth} columnWidth={RESULT_WIDTH} />
            </>
          )
        )}
      </View>
    </ScrollView>
  );
}

/** Suggestions or recent searches: picking one searches it. */
function Picks({ title, icon, items, onPick, action }: { title: string; icon: string; items: string[]; onPick: (value: string) => void; action?: ReactNode }) {
  return (
    <View style={{ gap: 4 }}>
      <View style={styles.picksHead}>
        <Text style={styles.picksTitle} numberOfLines={1}>
          {title}
        </Text>
        {action}
      </View>
      {items.map((item) => (
        <Focusable key={item} onPress={() => onPick(item)} accessibilityLabel={item} style={(active) => [styles.pick, active && styles.pickActive]}>
          {(active) => (
            <>
              <Icon d={icon} size={18} color={active ? '#000' : colors.muted} />
              <Text style={[styles.pickText, active && { color: '#000' }]} numberOfLines={1}>
                {item}
              </Text>
            </>
          )}
        </Focusable>
      ))}
    </View>
  );
}

/** The profile's last searches, newest first, kept on this device. */
function useSearchHistory(profileId?: number) {
  const key = `search.history.${profileId ?? 0}`;
  const [items, setItems] = useState<string[]>([]);
  useEffect(() => {
    getItem(key)
      .then((v) => setItems(v ? JSON.parse(v) : []))
      .catch(() => setItems([]));
  }, [key]);
  const save = (next: string[]) => {
    setItems(next);
    setItem(key, next.length ? JSON.stringify(next) : null).catch(() => {});
  };
  return {
    items,
    add: (value: string) => {
      const v = value.trim();
      if (v) save([v, ...items.filter((i) => i.toLowerCase() !== v.toLowerCase())].slice(0, HISTORY_MAX));
    },
    clear: () => save([]),
  };
}

const styles = StyleSheet.create({
  page: { flexDirection: 'row', alignItems: 'flex-start', gap: 32, paddingTop: 32, paddingBottom: 60 },
  pagePhone: { flexDirection: 'column', gap: 24 },
  panel: { gap: 20 },
  results: { flex: 1, gap: 16, alignSelf: 'stretch' },
  field: { fontSize: 20, backgroundColor: '#222', borderColor: '#333' },
  query: { flexDirection: 'row', alignItems: 'center', gap: 10, height: 48, paddingHorizontal: 12, borderBottomWidth: 2, borderBottomColor: '#555' },
  queryText: { flex: 1, color: '#fff', fontSize: 20, fontWeight: '600' },
  picksHead: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 8, minHeight: 32, marginBottom: 4 },
  picksTitle: { flexShrink: 1, color: colors.muted, fontSize: 13, fontWeight: '700', textTransform: 'uppercase', letterSpacing: 1 },
  pick: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 8, paddingHorizontal: 10, borderRadius: 4 },
  pickActive: { backgroundColor: '#fff' },
  pickText: { flex: 1, color: '#ddd', fontSize: 17 },
});
