// PairingScreen — "Add a Mac". Manual host+token entry (works on the simulator), or
// Scan, which opens ScanScreen (react-native-camera-kit, a real device's camera) for the
// pairing QR that `gtmux pair` / `gtmux tunnel` print.

import React, {useEffect, useRef, useState} from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Modal,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaProvider, SafeAreaView} from 'react-native-safe-area-context';
import {GtmuxClient} from '../api/client';
import {useApp} from '../state/AppContext';
import {EnrollError, PairedMac, enrollAndSave, normalizeHost, parsePairingQR, parseShareLink, redeemShareCodeAndSave} from '../pairing/qr';
import {checkServer} from '../pairing/deadline';
import {deviceLabel} from '../pairing/deviceName';
import {BrandMark} from '../ui/BrandMark';
import {StatusColor} from '../ui/theme';
import {ScanScreen} from './ScanScreen';
import {TestIds} from '../constants/testIds';
import {MODAL_ORIENTATIONS} from '../ui/modalOrientations';

// A share link's code as `gtmux share new` prints it: eight letters and digits in two
// groups of four, with or without the dash (internal/server/sharecode.go). A token may
// have this shape too (`serve --token` takes any text), so it only decides what to try
// after the token is refused.
const SHARE_CODE = /^[0-9a-z]{4}-?[0-9a-z]{4}$/i;

// thisDeviceLabel names this phone in the server's device roster (so you can tell devices
// apart and revoke the right one). The rule lives in pairing/deviceName so it can be
// tested off-device; here we only read what the platform knows about itself.
function thisDeviceLabel(): string {
  const c = Platform.constants as any;
  return deviceLabel(Platform.OS, c?.interfaceIdiom);
}

// onCancel, when provided, renders a Cancel control — set when PairingScreen is
// presented as the "Add a Mac" sheet from ServersScreen (vs. the bare first run).
export function PairingScreen({onCancel, onDemo, onPaired}: {onCancel?: () => void; onDemo?: () => void; onPaired?: () => void} = {}) {
  const {t, pal, pair, lang, activeUrl} = useApp();
  const [host, setHost] = useState('');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [scanning, setScanning] = useState(false);

  const scanEpoch = useRef(0);
  const scanSession = scanEpoch.current;
  const operation = useRef(0);
  const running = useRef(false);
  const alive = useRef(true);
  const active = useRef(activeUrl);
  active.current = activeUrl;
  const pendingSave = useRef<PairedMac | null>(null);
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; };
  }, []);

  useEffect(() => {
    // Even switching away and back cancels the original target's pending response.
    operation.current++;
    running.current = false;
    pendingSave.current = null;
    setBusy(false);
    setSaving(false);
  }, [activeUrl]);

  const cancel = () => {
    scanEpoch.current++;
    operation.current++;
    running.current = false;
    pendingSave.current = null;
    onCancel?.();
  };

  // One admission gate for camera callbacks, manual taps and retries. React state
  // alone cannot lock two callbacks delivered before the next render.
  const run = async (work: (save: (m: PairedMac) => Promise<void>, current: () => boolean) => Promise<void>, share = false) => {
    if (running.current || !alive.current) return;
    running.current = true;
    const id = ++operation.current;
    const origin = active.current;
    const current = () => alive.current && operation.current === id && active.current === origin;
    setBusy(true);
    setError('');
    const save = async (m: PairedMac) => {
      if (!current()) return;
      pendingSave.current = m;
      setSaving(true);
      try {
        // Check again when the serialized Keychain transaction starts. A previous
        // write may still be finishing when the user changes their target.
        await pair(m, current);
        pendingSave.current = null;
        if (alive.current && operation.current === id) onPaired?.();
      } catch {
        throw new Error('pairSaveFailed');
      } finally {
        if (alive.current && operation.current === id) setSaving(false);
      }
    };
    try {
      await work(save, current);
    } catch (e: any) {
      if (!current()) return;
      if (e?.message === 'pairSaveFailed') {
        setError(t('pairSaveFailed'));
      } else if (e instanceof EnrollError) {
        setError(t(e.kind === 'unreachable' ? 'enrollUnreachable'
          : e.kind === 'tunnelDown' ? (share ? 'shareMacDown' : 'enrollTunnelDown')
          : e.kind === 'noToken' ? (share ? 'shareNoToken' : 'enrollNoToken')
          : share ? (e.status === 429 ? 'shareTooMany' : 'shareRefused') : 'enrollCodeInvalid'));
      } else {
        setError(t('badToken'));
      }
    } finally {
      if (operation.current === id) {
        running.current = false;
        if (alive.current) setBusy(false);
      }
    }
  };

  const connectWith = (base: string, tok: string, name: string, scope: 'owner' | 'guest' = 'owner', maybeCode = false) =>
    run(async (save, current) => {
      const found = await checkServer(new GtmuxClient(base, tok));
      if (!current()) return;
      if (found === 'ok') {
        await save({url: base, token: tok, name, scope});
      } else if (found === 'rejected' && maybeCode) {
        await redeemShareCodeAndSave({kind: 'guestCode', url: base, code: tok, name}, thisDeviceLabel(), save);
      } else {
        setError(t(found === 'unreachable' ? 'cantReach' : 'badToken'));
      }
    }, maybeCode);

  const connect = () => {
    if (running.current) return;
    // A code already redeemed must never be submitted again merely because the
    // Keychain was locked. Retry saving the issued credential in this sheet.
    const pending = pendingSave.current;
    if (pending) { void run(save => save(pending)); return; }
    const guest = parseShareLink(host.trim());
    if (guest?.kind === 'guestCode') {
      void run(save => redeemShareCodeAndSave(guest, thisDeviceLabel(), save), true);
      return;
    }
    if (guest) { void connectWith(guest.url, guest.token, guest.name, 'guest'); return; }
    const base = normalizeHost(host);
    if (!base || !token.trim()) {
      setError(t(!base ? 'pairNeedAddress' : 'pairNeedToken'));
      return;
    }
    void connectWith(base, token.trim(), base.replace(/^https?:\/\//, ''), 'owner', SHARE_CODE.test(token.trim()));
  };

  const onScanned = (raw: string) => {
    if (running.current || !alive.current) return;
    setScanning(false);
    let res;
    try { res = parsePairingQR(raw); }
    catch { setError(t('pairInvalidQR')); return; }
    // A deliberate new scan replaces a failed save's retry target.
    pendingSave.current = null;
    if (res.kind === 'paired' || res.kind === 'guest') {
      void connectWith(res.url, res.token, res.name, res.kind === 'guest' ? 'guest' : 'owner');
    } else if (res.kind === 'guestCode') {
      const shared = res;
      void run(save => redeemShareCodeAndSave(shared, thisDeviceLabel(), save), true);
    } else {
      const owner = res;
      void run(save => enrollAndSave(owner, thisDeviceLabel(), save));
    }
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} testID={TestIds.pairing.screen}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={styles.flex}>
        {onCancel && (
          <TouchableOpacity onPress={cancel} disabled={saving} hitSlop={hitC} style={styles.cancel}>
            <Text style={[styles.cancelText, {color: pal.fg2}]}>‹ {t('cancel')}</Text>
          </TouchableOpacity>
        )}
        <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
          <BrandMark size={48} neutral={pal.fg3} />
          <Text style={[styles.brand, {color: pal.fg}]}>gtmux</Text>
          <Text style={[styles.title, {color: pal.fg}]}>{t('addMac')}</Text>
          <Text style={[styles.subtitle, {color: pal.fg2}]}>
            {lang === 'zh'
              ? '在服务器上运行 gtmux pair，扫码连接。也可以手动输入地址和 token。'
              : 'Run gtmux pair on your server and scan the code. Or enter the address and token below.'}
          </Text>

          <TouchableOpacity
            testID={TestIds.pairing.scan}
            accessibilityLabel={t('scanQR')}
            activeOpacity={0.85}
            style={styles.qrBtn}
            disabled={busy}
            onPress={() => {
              if (running.current) return;
              setError('');
              scanEpoch.current++;
              setScanning(true);
            }}>
            <Text style={styles.qrText}>◉  {t('scanQR')}</Text>
          </TouchableOpacity>

          <Text style={[styles.or, {color: pal.fg3}]}>—— {t('manualEntry')} ——</Text>

          <Text style={[styles.label, {color: pal.fg2}]}>{t('host')}</Text>
          <TextInput
            testID={TestIds.pairing.host}
            editable={!busy && !pendingSave.current}
            value={host}
            onChangeText={setHost}
            placeholder="192.168.1.20:8765"
            placeholderTextColor={pal.fg3}
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="url"
            style={[styles.input, {color: pal.fg, borderColor: pal.divider, backgroundColor: pal.surface}]}
          />

          <Text style={[styles.label, {color: pal.fg2}]}>{t('token')}</Text>
          <TextInput
            testID={TestIds.pairing.token}
            editable={!busy && !pendingSave.current}
            value={token}
            onChangeText={setToken}
            placeholder="serve-token"
            placeholderTextColor={pal.fg3}
            autoCapitalize="none"
            autoCorrect={false}
            secureTextEntry
            style={[styles.input, {color: pal.fg, borderColor: pal.divider, backgroundColor: pal.surface}]}
          />

          {!!error && (
            <Text testID={TestIds.pairing.error} style={styles.error}>
              {error}
            </Text>
          )}

          <TouchableOpacity
            testID={TestIds.pairing.connect}
            accessibilityLabel={t('connect')}
            style={[styles.connect, busy && styles.connectBusy]}
            onPress={connect}
            disabled={busy}>
            {busy ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.connectText}>{t(pendingSave.current ? 'pairSaveRetry' : 'connect')}</Text>
            )}
          </TouchableOpacity>

          {/* Escape hatch for someone without a Mac handy (e.g. an App Store
              reviewer): a clickable tour with sample data. A badged SECONDARY CARD
              (F7① — the old dim link was invisible), still clearly subordinate to
              the two real pairing paths above. */}
          {onDemo && (
            <TouchableOpacity
              disabled={busy}
              onPress={() => { cancel(); onDemo(); }}
              style={[styles.demoCard, {borderColor: pal.divider, backgroundColor: pal.surface}]}
              accessibilityRole="button"
              accessibilityLabel={lang === 'zh' ? '查看演示' : 'View demo'}>
              <View style={[styles.demoBadge, {borderColor: StatusColor.working}]}>
                <Text style={[styles.demoBadgeText, {color: StatusColor.working}]}>DEMO</Text>
              </View>
              <View style={styles.demoBody}>
                <Text style={[styles.demoTitle, {color: pal.fg}]}>
                  {lang === 'zh' ? '查看演示' : 'View demo'}
                </Text>
                <Text style={[styles.demoSub, {color: pal.fg3}]}>
                  {lang === 'zh' ? '样例数据 · 无需任何服务器' : 'Sample data · no server needed'}
                </Text>
              </View>
              <Text style={[styles.demoChevron, {color: pal.fg3}]}>›</Text>
            </TouchableOpacity>
          )}
        </ScrollView>
      </KeyboardAvoidingView>
      {/* A Modal renders OUTSIDE the app's SafeAreaProvider, so the scanner's own
          SafeAreaView would see 0 insets and its title would sit under the Dynamic
          Island. Re-establish the provider inside the modal so the top inset is real. */}
      <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={scanning} animationType="slide" onRequestClose={() => { scanEpoch.current++; setScanning(false); }}>
        <SafeAreaProvider>
          {scanning && <ScanScreen
            onClose={() => { scanEpoch.current++; setScanning(false); }}
            onScanned={raw => {
              if (scanEpoch.current !== scanSession) return;
              scanEpoch.current++;
              onScanned(raw);
            }}
          />}
        </SafeAreaProvider>
      </Modal>
    </SafeAreaView>
  );
}

const hitC = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  flex: {flex: 1},
  cancel: {paddingHorizontal: 16, paddingTop: 12, paddingBottom: 2},
  cancelText: {fontSize: 16, fontWeight: '500'},
  container: {padding: 24, paddingTop: 18},
  brand: {fontSize: 15, fontWeight: '700', opacity: 0.6, marginTop: 14, marginBottom: 4},
  title: {fontSize: 28, fontWeight: '700', marginBottom: 10},
  subtitle: {fontSize: 13.5, lineHeight: 19, marginBottom: 26},
  // The primary path — a filled accent button so it clearly reads as tappable.
  qrBtn: {
    backgroundColor: '#06B6D4',
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: 'center',
  },
  qrText: {fontSize: 16, fontWeight: '700', color: '#fff'},
  or: {textAlign: 'center', marginVertical: 22, fontSize: 12},
  label: {fontSize: 12, fontWeight: '600', marginBottom: 6},
  input: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    paddingHorizontal: 14,
    paddingVertical: 12,
    fontSize: 16,
    marginBottom: 18,
  },
  error: {color: '#EF4444', marginBottom: 14, fontSize: 13},
  connect: {
    backgroundColor: '#06B6D4',
    borderRadius: 12,
    paddingVertical: 15,
    alignItems: 'center',
    marginTop: 6,
  },
  connectBusy: {opacity: 0.7},
  connectText: {color: '#fff', fontSize: 16, fontWeight: '700'},
  demoCard: {
    flexDirection: 'row',
    alignItems: 'center',
    marginTop: 18,
    padding: 12,
    borderRadius: 12,
    borderWidth: StyleSheet.hairlineWidth,
  },
  demoBadge: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 6, paddingVertical: 1},
  demoBadgeText: {fontSize: 10, fontWeight: '700', letterSpacing: 0.06},
  demoBody: {flex: 1, marginLeft: 10},
  demoTitle: {fontSize: 14, fontWeight: '600'},
  demoSub: {fontSize: 12, marginTop: 1},
  demoChevron: {fontSize: 17, fontWeight: '300', marginLeft: 8},
});
