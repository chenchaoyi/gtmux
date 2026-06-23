/* eslint-env jest */
/* global jest */
// Mocks for the native modules the app imports, so component/smoke tests can run
// under jest (Node) without a native runtime. Pure-logic tests don't need these.

// Navigation: render children directly and stub screens — sidesteps the native
// react-native-screens stack entirely for a lightweight App smoke test.
jest.mock('@react-navigation/native', () => {
  const ref = {isReady: () => false, navigate: jest.fn(), current: null, resetRoot: jest.fn()};
  return {
    NavigationContainer: ({children}) => children,
    useNavigation: () => ({navigate: jest.fn(), goBack: jest.fn()}),
    useNavigationContainerRef: () => ref,
    createNavigationContainerRef: () => ref,
    DefaultTheme: {colors: {}},
    DarkTheme: {colors: {}},
  };
});
jest.mock('@react-navigation/native-stack', () => ({
  createNativeStackNavigator: () => ({
    Navigator: ({children}) => children,
    Screen: () => null,
  }),
}));

jest.mock('react-native-safe-area-context', () => {
  // the shipped mock exposes components under `default`; re-export them as named
  // too, since the app imports {SafeAreaProvider, SafeAreaView}.
  const m = require('react-native-safe-area-context/jest/mock');
  const base = m.default || m;
  return {__esModule: true, ...base, default: base};
});

jest.mock('@react-native-async-storage/async-storage', () => ({
  setItem: jest.fn(() => Promise.resolve()),
  getItem: jest.fn(() => Promise.resolve(null)),
  removeItem: jest.fn(() => Promise.resolve()),
}));

jest.mock('react-native-keychain', () => ({
  setGenericPassword: jest.fn(() => Promise.resolve()),
  getGenericPassword: jest.fn(() => Promise.resolve(false)),
  resetGenericPassword: jest.fn(() => Promise.resolve()),
}));

jest.mock('react-native-sse', () => {
  return class EventSource {
    addEventListener() {}
    removeAllEventListeners() {}
    close() {}
  };
});

jest.mock('@react-native-clipboard/clipboard', () => ({
  __esModule: true,
  default: {
    getString: jest.fn(() => Promise.resolve('')),
    setString: jest.fn(),
    hasImage: jest.fn(() => Promise.resolve(false)),
    getImagePNG: jest.fn(() => Promise.resolve('')),
  },
}));

jest.mock('react-native-image-picker', () => ({
  launchCamera: jest.fn(() => Promise.resolve({assets: []})),
  launchImageLibrary: jest.fn(() => Promise.resolve({assets: []})),
}));

jest.mock('@react-native-documents/picker', () => ({pick: jest.fn(() => Promise.resolve([]))}));

jest.mock('react-native-camera-kit', () => ({Camera: () => null}));

jest.mock('react-native-view-shot', () => ({captureRef: jest.fn(() => Promise.resolve('file://markup.png'))}));

jest.mock('@react-native-community/push-notification-ios', () => ({
  __esModule: true,
  default: {
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    requestPermissions: jest.fn(() => Promise.resolve({alert: true})),
    getInitialNotification: jest.fn(() => Promise.resolve(null)),
  },
  FetchResult: {NoData: 'noData'},
}));

// react-native-svg → plain host components so SVG-using components render in jest.
jest.mock('react-native-svg', () => {
  const React = require('react');
  const stub = name => props => React.createElement(name, props, props && props.children);
  const names = ['Svg', 'Path', 'Rect', 'Circle', 'G', 'Line', 'Text', 'Defs', 'LinearGradient', 'Stop', 'Polygon', 'Polyline', 'Ellipse'];
  const out = {__esModule: true, default: stub('Svg')};
  names.forEach(n => (out[n] = stub(n)));
  return out;
});
