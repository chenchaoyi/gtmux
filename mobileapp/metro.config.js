const { getDefaultConfig, mergeConfig } = require('@react-native/metro-config');

/**
 * Metro configuration
 * https://reactnative.dev/docs/metro
 *
 * @type {import('@react-native/metro-config').MetroConfig}
 */
// Tests run in Jest. A product import of a test is an error, never a bundle input.
// CI scope narrowing pins this complete configuration; changes retain native gates.
const config = {resolver: {blockList: /[/\\][^/\\]+\.test\.[jt]sx?$/}};

module.exports = mergeConfig(getDefaultConfig(__dirname), config);
