import { Slot } from 'expo-router';
import { View } from 'react-native';
import { Sidebar } from '../../components/Sidebar';
import { Gate } from '../../lib/auth';
import { colors, useLayout } from '../../theme';

/** Every browsing screen: navigation rail (or tab bar on phones) around the content. */
export default function AppLayout() {
  const { rail, tabBar } = useLayout();
  return (
    <Gate>
      <View style={{ flex: 1, backgroundColor: colors.bg }}>
        <View style={{ flex: 1, paddingLeft: rail, paddingBottom: tabBar }}>
          <Slot />
        </View>
        <Sidebar />
      </View>
    </Gate>
  );
}
