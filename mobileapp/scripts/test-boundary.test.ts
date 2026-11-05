jest.mock('@react-native/metro-config', () => ({
  getDefaultConfig: () => ({ resolver: {} }),
  mergeConfig: (base: object, extra: object) => ({ ...base, ...extra }),
}));
test('actual Metro resolver refuses test modules while preserving production and native variant sources', () => {
  const {
    resolver: { blockList },
  } = require('../metro.config.js');
  for (const file of [
    '/repo/src/screens/HQ.test.tsx',
    'C:\\repo\\src\\state\\drafts.test.ts',
    '/repo/helper.test.js',
  ])
    expect(blockList.test(file)).toBe(true);
  for (const file of [
    '/repo/src/screens/HQ.tsx',
    '/repo/src/widget.ios.ts',
    '/repo/src/test-data.ts',
    '/repo/e2e/fake-serve/server.ts',
  ])
    expect(blockList.test(file)).toBe(false);
});
