import { useQuery } from '@tanstack/react-query';
import { ScrollView, Text, View } from 'react-native';
import { api, unwrap } from '../../api/client';
import { useLibrary } from '../../components/Browse';
import { PosterGrid, progressCard, Row, styles as ui } from '../../components/ui';
import type { Progress } from '../../api/client';
import { t } from '../../i18n';
import { useLayout } from '../../theme';

/**
 * In progress (most recent first, unfinished), then new episodes of the shows caught up with, then the titles of
 * the list not started yet.
 */
export default function MyList() {
  const { list, progress } = useLibrary();
  const { gutter } = useLayout();
  const family = useQuery({ queryKey: ['library', 'family'], queryFn: () => unwrap(api.GET('/library/family')) });
  const ratings = useQuery({ queryKey: ['library', 'ratings'], queryFn: () => unwrap(api.GET('/library/ratings')) });
  const finished = useQuery({ queryKey: ['library', 'finished'], queryFn: () => unwrap(api.GET('/library/finished')) });
  const liked = (ratings.data ?? []).filter((r) => r.rating > 0);
  const watched = useQuery({ queryKey: ['library', 'watched'], queryFn: () => unwrap(api.GET('/library/watched')) });
  // "Continue watching" holds both: titles in progress, and next episodes that aired (they have a badge).
  const started = progress.filter((p) => !p.badge);
  const fresh = progress.filter((p) => p.badge);
  // Not started: neither in the rows above nor ever played (a finished title is not "to watch").
  const played = new Set([...progress, ...(watched.data ?? [])].map((p) => `${p.type}-${p.id}`));
  const rest = list.filter((i) => !played.has(`${i.type}-${i.id}`));
  const key = (p: Progress) => `${p.type}-${p.id}`;

  return (
    <ScrollView style={ui.page} contentContainerStyle={{ paddingTop: 48, paddingBottom: 60 }}>
      <Text style={[ui.h1, { paddingHorizontal: gutter }]}>{t('common.myList')}</Text>
      {started.length > 0 && <Row<Progress> title={t('list.inProgress')} data={started} keyOf={key} render={progressCard} />}
      {fresh.length > 0 && <Row<Progress> title={t('list.newEpisodes')} data={fresh} keyOf={key} render={progressCard} />}
      <View style={{ paddingHorizontal: gutter, gap: 12, marginTop: 12 }}>
        {(started.length > 0 || fresh.length > 0) && rest.length > 0 && <Text style={ui.h2}>{t('list.toWatch')}</Text>}
        {rest.length > 0 ? <PosterGrid items={rest} /> : !progress.length && <Text style={ui.muted}>{t('browse.myListEmpty')}</Text>}
        {liked.length > 0 && (
          <>
            <Text style={[ui.h2, { marginTop: 24 }]}>{t('list.liked')}</Text>
            <PosterGrid items={liked} />
          </>
        )}
        {!!family.data?.length && (
          <>
            <Text style={[ui.h2, { marginTop: 24 }]}>{t('list.family')}</Text>
            <PosterGrid items={family.data} />
          </>
        )}
        {!!finished.data?.length && (
          <>
            <Text style={[ui.h2, { marginTop: 24 }]}>{t('list.finished')}</Text>
            <PosterGrid items={finished.data} />
          </>
        )}
      </View>
    </ScrollView>
  );
}
