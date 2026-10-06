import { router, type Href } from 'expo-router';
import { useState, type ReactNode, type Ref } from 'react';
import { Platform, Pressable, type View, type NativeSyntheticEvent, type PressableProps, type StyleProp, type TargetedEvent, type ViewStyle } from 'react-native';

type Props = Omit<PressableProps, 'style' | 'children'> & {
  /** Navigates on press. */
  href?: Href;
  ref?: Ref<View>;
  style?: StyleProp<ViewStyle> | ((active: boolean) => StyleProp<ViewStyle>);
  children: ReactNode | ((active: boolean) => ReactNode);
};

/**
 * The one interactive primitive of the app. "active" means focused (TV remote, keyboard) or hovered (mouse),
 * so every screen looks the same whatever the input: D-pad on Android TV/tvOS, keyboard or mouse on the web.
 */
export function Focusable({ href, style, children, onPress, onFocus, onBlur, onHoverIn, onHoverOut, ...rest }: Props) {
  const [focused, setFocused] = useState(false);
  const [hovered, setHovered] = useState(false);
  const active = focused || hovered;

  return (
    <Pressable
      // "button" even for navigation: react-native-web only activates button roles with Enter/Space,
      // which is what a TV remote and keyboard users expect.
      accessibilityRole="button"
      {...rest}
      onPress={(e) => {
        onPress?.(e);
        if (href) router.push(href);
      }}
      onFocus={(e) => {
        if (!keyboardFocus(e)) return;
        setFocused(true);
        onFocus?.(e);
      }}
      onBlur={(e) => {
        setFocused(false);
        onBlur?.(e);
      }}
      onHoverIn={(e) => {
        setHovered(true);
        onHoverIn?.(e);
      }}
      onHoverOut={(e) => {
        setHovered(false);
        onHoverOut?.(e);
      }}
      style={[{ outlineWidth: 0, outlineStyle: 'solid' }, typeof style === 'function' ? style(active) : style]}
    >
      {typeof children === 'function' ? children(active) : children}
    </Pressable>
  );
}

/**
 * On the web a click also focuses what was clicked: it must not count, or the sidebar and the cards would stay
 * highlighted until the next click. Only keyboard focus does, the one browsers draw their focus ring for.
 */
function keyboardFocus(e: NativeSyntheticEvent<TargetedEvent>) {
  if (Platform.OS !== 'web') return true; // TV remote and phones: every focus is meant
  const el = e.target as unknown as Element | null;
  return typeof el?.matches !== 'function' || el.matches(':focus-visible');
}
