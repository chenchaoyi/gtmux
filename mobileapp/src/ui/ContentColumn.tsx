// ContentColumn — caps content to a readable width and centers it on wide screens
// (iPad / landscape), so phone-derived full-width lists don't stretch edge-to-edge
// with a label on the left and its value ~1000pt away on the right. On a phone
// (width < maxWidth) it's a no-op: width 100%, nothing capped.

import React from 'react';
import {StyleProp, View, ViewStyle} from 'react-native';

export function ContentColumn({
  children,
  max = 600,
  style,
}: {
  children: React.ReactNode;
  max?: number;
  style?: StyleProp<ViewStyle>;
}) {
  return <View style={[{width: '100%', maxWidth: max, alignSelf: 'center'}, style]}>{children}</View>;
}
