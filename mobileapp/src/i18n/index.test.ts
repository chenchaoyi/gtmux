import {Platform, NativeModules} from 'react-native';
import {resolveLang, makeT, statusLabel, deviceLang} from './index';

describe('deviceLang', () => {
  const origOS = Platform.OS;
  const origSettings = NativeModules.SettingsManager;
  const origI18n = NativeModules.I18nManager;

  afterEach(() => {
    // Restore mutated globals between cases.
    (Platform as any).OS = origOS;
    NativeModules.SettingsManager = origSettings;
    NativeModules.I18nManager = origI18n;
  });

  it('reads AppleLocale on iOS and maps zh* -> zh', () => {
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = {settings: {AppleLocale: 'zh_CN'}};
    expect(deviceLang()).toBe('zh');
  });

  it('falls back to AppleLanguages[0] on iOS', () => {
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = {settings: {AppleLanguages: ['zh-Hans']}};
    expect(deviceLang()).toBe('zh');
  });

  it('maps any non-zh iOS locale to en', () => {
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = {settings: {AppleLocale: 'en_US'}};
    expect(deviceLang()).toBe('en');
  });

  it('reads I18nManager.localeIdentifier on android', () => {
    (Platform as any).OS = 'android';
    NativeModules.I18nManager = {localeIdentifier: 'zh_CN_#Hans'};
    expect(deviceLang()).toBe('zh');
    NativeModules.I18nManager = {localeIdentifier: 'fr_FR'};
    expect(deviceLang()).toBe('en');
  });

  it('defaults to en when native modules are missing/throw', () => {
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = undefined;
    expect(deviceLang()).toBe('en');
    (Platform as any).OS = 'android';
    NativeModules.I18nManager = undefined;
    expect(deviceLang()).toBe('en');
  });

  it('is case-insensitive on the zh prefix', () => {
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = {settings: {AppleLocale: 'ZH-Hant'}};
    expect(deviceLang()).toBe('zh');
  });
});

describe('resolveLang', () => {
  it('returns the explicit preference as-is', () => {
    expect(resolveLang('en')).toBe('en');
    expect(resolveLang('zh')).toBe('zh');
  });

  it('resolves "system" via deviceLang', () => {
    const origOS = Platform.OS;
    const origSettings = NativeModules.SettingsManager;
    (Platform as any).OS = 'ios';
    NativeModules.SettingsManager = {settings: {AppleLocale: 'zh_CN'}};
    expect(resolveLang('system')).toBe('zh');
    NativeModules.SettingsManager = {settings: {AppleLocale: 'en_US'}};
    expect(resolveLang('system')).toBe('en');
    (Platform as any).OS = origOS;
    NativeModules.SettingsManager = origSettings;
  });
});

describe('makeT', () => {
  it('looks up a known key in en', () => {
    const t = makeT('en');
    expect(t('connect')).toBe('Connect');
    expect(t('needsYou')).toBe('NEEDS YOU');
  });

  it('looks up a known key in zh', () => {
    const t = makeT('zh');
    expect(t('connect')).toBe('连接');
    expect(t('needsYou')).toBe('需要你');
  });

  it('returns the key string itself for an unknown key', () => {
    const t = makeT('en');
    expect(t('nope' as any)).toBe('nope');
    expect(t('' as any)).toBe('');
  });

  it('produces independent lookups per language', () => {
    expect(makeT('en')('settings')).toBe('Settings');
    expect(makeT('zh')('settings')).toBe('设置');
  });
});

describe('statusLabel', () => {
  const cases: Array<[string, string, string]> = [
    ['waiting', 'waiting', '等输入'],
    ['working', 'working', '运行中'],
    ['idle', 'idle', '空闲'],
    ['running', 'running', '待命'],
  ];

  it.each(cases)('maps %s in en and zh', (status, en, zh) => {
    expect(statusLabel(status, 'en')).toBe(en);
    expect(statusLabel(status, 'zh')).toBe(zh);
  });

  it('returns the raw status for an unknown status', () => {
    expect(statusLabel('bogus', 'en')).toBe('bogus');
    expect(statusLabel('bogus', 'zh')).toBe('bogus');
  });

  it('returns "" for an empty status', () => {
    expect(statusLabel('', 'en')).toBe('');
  });
});
