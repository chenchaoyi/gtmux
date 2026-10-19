import React from 'react';
import {Image} from 'react-native';

// UIKit and RN use the same generated, scale-aware artwork. The ink ground keeps
// every pane visible in light mode; the focused pane retains the brand cyan.
export function LaunchMark() {
  return (
    <Image
      source={require('../../ios/GtmuxMobile/Images.xcassets/LaunchMark.imageset/launch-mark.png')}
      style={{width: 112, height: 112}}
      resizeMode="contain"
      accessible={false}
      testID="launch-mark"
    />
  );
}
