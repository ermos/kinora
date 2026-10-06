import { Slot } from 'expo-router';
import { useRef, type ComponentRef } from 'react';
import { Platform, TVFocusGuideView, View } from 'react-native';
import { Sidebar } from '../../components/Sidebar';
import { Gate } from '../../lib/auth';
import { colors, useLayout } from '../../theme';

/** TV: entering the content goes to its first focusable item, or the last one focused when coming back to it. */
const Content = Platform.isTV ? TVFocusGuideView : View;

/** Every browsing screen: navigation rail (or tab bar on phones) around the content. */
export default function AppLayout() {
  const { rail, tabBar } = useLayout();
  const content = useRef<ComponentRef<typeof TVFocusGuideView>>(null);
  return (
    <Gate>
      <View style={{ flex: 1, backgroundColor: colors.bg }}>
        <Content ref={content} autoFocus style={{ flex: 1, paddingLeft: rail, paddingBottom: tabBar }}>
          <Slot />
        </Content>
        <Sidebar content={content} />
      </View>
    </Gate>
  );
}
