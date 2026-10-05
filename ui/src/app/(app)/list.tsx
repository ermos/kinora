import { Text } from 'react-native';
import { Page, useLibrary } from '../../components/Browse';
import { PosterGrid, styles as ui } from '../../components/ui';
import { t } from '../../i18n';

export default function MyList() {
  const { list } = useLibrary();
  return (
    <Page>
      <Text style={ui.h1}>{t('common.myList')}</Text>
      {list.length ? <PosterGrid items={list} /> : <Text style={ui.muted}>{t('browse.myListEmpty')}</Text>}
    </Page>
  );
}
