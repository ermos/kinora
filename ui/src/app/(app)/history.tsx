import { Text, View } from 'react-native';
import { Page } from '../../components/Browse';
import { HistoryList } from '../../components/WatchHistory';
import { styles as ui } from '../../components/ui';
import { t } from '../../i18n';

export default function History() {
  return (
    <Page>
      <View style={{ maxWidth: 900, gap: 24 }}>
        <Text style={[ui.h1, { marginBottom: 0 }]}>{t('history.title')}</Text>
        <HistoryList />
      </View>
    </Page>
  );
}
