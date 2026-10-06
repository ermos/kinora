import { useQuery } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Platform, Text, type TextInput } from 'react-native';
import { api, unwrap } from '../../api/client';
import { Page } from '../../components/Browse';
import { Field, PosterGrid, Spinner, styles as ui } from '../../components/ui';
import { t } from '../../i18n';

export default function Search() {
  const params = useLocalSearchParams<{ q?: string }>();
  const q = params.q ?? '';
  const [input, setInput] = useState(q);
  const debounce = useRef<ReturnType<typeof setTimeout>>(undefined);
  const field = useRef<TextInput>(null);
  // TV: Field skips autoFocus (the keyboard would cover forms), here typing is the point. Taking the focus also moves
  // it out of the sidebar, which closes.
  useEffect(() => {
    if (Platform.isTV) field.current?.focus();
  }, []);
  const results = useQuery({
    queryKey: ['search', q],
    queryFn: () => unwrap(api.GET('/catalog/search', { params: { query: { q } } })),
    enabled: q !== '',
  });

  const onChange = (value: string) => {
    setInput(value);
    clearTimeout(debounce.current);
    debounce.current = setTimeout(() => router.setParams({ q: value.trim() || undefined }), 350);
  };

  return (
    <Page>
      <Field value={input} onChangeText={onChange} placeholder={t('browse.searchPlaceholder')} ref={field} autoFocus accessibilityLabel={t('nav.search')} style={{ fontSize: 24, maxWidth: 720, marginBottom: 24, backgroundColor: '#222', borderColor: '#333' }} />
      {!q ? null : results.isPending ? <Spinner /> : results.data?.length ? <PosterGrid items={results.data} /> : <Text style={ui.muted}>{t('browse.noResults', { q })}</Text>}
    </Page>
  );
}
