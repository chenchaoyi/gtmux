// PairingScreen — "Add a Mac". Manual host+token entry (works on the simulator).
// The QR scanner needs react-native-vision-camera + a real device; that's a
// later increment, so the Scan button explains it for now.

import React, {useState} from 'react';
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
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {GtmuxClient} from '../api/client';
import {useApp} from '../state/AppContext';
import {EnrollError, enrollDevice, normalizeHost, parsePairingQR, parseShareLink} from '../pairing/qr';
import {BrandMark} from '../ui/BrandMark';
import {ScanScreen} from './ScanScreen';
import {TestIds} from '../constants/testIds';

// deviceLabel names this phone in the Mac's device roster (so you can tell devices
// apart and revoke the right one).
function deviceLabel(): string {
  return Platform.OS === 'ios' ? 'gtmux • iPhone' : 'gtmux • Android';
}

// onCancel, when provided, renders a Cancel control — set when PairingScreen is
// presented as the "Add a Mac" sheet from ServersScreen (vs. the bare first run).
export function PairingScreen({onCancel, onDemo}: {onCancel?: () => void; onDemo?: () => void} = {}) {
  const {t, pal, pair, lang} = useApp();
  const [host, setHost] = useState('');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [scanning, setScanning] = useState(false);

  // connectWith validates reachability + token, then pairs. Shared by manual
  // entry and the QR scanner.
  const connectWith = async (
    base: string,
    tok: string,
    name: string,
    scope: 'owner' | 'guest' = 'owner',
  ) => {
    if (!base || !tok) {
      setError(t('cantReach'));
      return;
    }
    setBusy(true);
    setError('');
    try {
      const client = new GtmuxClient(base, tok);
      if (!(await client.health())) {
        setError(t('cantReach'));
        return;
      }
      await client.agents(); // validate the token with a real authed call (a guest token is accepted too)
      await pair({url: base, token: tok, name, scope});
    } catch {
      setError(t('badToken'));
    } finally {
      setBusy(false);
    }
  };

  const connect = () => {
    // A pasted guest link (`<base>/#g=<token>`, legacy `#t=`) → scope-restricted guest.
    const guest = parseShareLink(host.trim());
    if (guest) {
      connectWith(guest.url, guest.token, guest.name, 'guest');
      return;
    }
    const base = normalizeHost(host);
    connectWith(base, token.trim(), base.replace(/^https?:\/\//, ''));
  };

  const onScanned = async (raw: string) => {
    setScanning(false);
    let res;
    try {
      res = parsePairingQR(raw);
    } catch (e: any) {
      setError(e?.message || t('badToken'));
      return;
    }
    if (res.kind === 'paired') {
      connectWith(res.url, res.token, res.name); // v1: token in the QR
      return;
    }
    if (res.kind === 'guest') {
      // A `gtmux share` guest link/QR → connect as a scope-restricted guest (no enroll).
      connectWith(res.url, res.token, res.name, 'guest');
      return;
    }
    // v2: redeem the one-time code for this device's own token, then connect.
    setBusy(true);
    setError('');
    try {
      const deviceToken = await enrollDevice(res.url, res.enrollCode, deviceLabel());
      setBusy(false);
      await connectWith(res.url, deviceToken, res.name);
    } catch (e: any) {
      setBusy(false);
      // Map the classified enroll failure to a precise, actionable message — a dead
      // link/tunnel is NOT an expired code, so point at the right thing to check.
      if (e instanceof EnrollError) {
        setError(
          t(
            e.kind === 'unreachable'
              ? 'enrollUnreachable'
              : e.kind === 'tunnelDown'
                ? 'enrollTunnelDown'
                : e.kind === 'noToken'
                  ? 'enrollNoToken'
                  : 'enrollCodeInvalid',
          ),
        );
      } else {
        setError(e?.message || t('badToken'));
      }
    }
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} testID={TestIds.pairing.screen}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={styles.flex}>
        {onCancel && (
          <TouchableOpacity onPress={onCancel} hitSlop={hitC} style={styles.cancel}>
            <Text style={[styles.cancelText, {color: pal.fg2}]}>‹ {t('cancel')}</Text>
          </TouchableOpacity>
        )}
        <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
          <BrandMark size={48} neutral={pal.fg3} />
          <Text style={[styles.brand, {color: pal.fg}]}>gtmux</Text>
          <Text style={[styles.title, {color: pal.fg}]}>{t('addMac')}</Text>
          <Text style={[styles.subtitle, {color: pal.fg3}]}>
            {lang === 'zh'
              ? '在服务器上跑 gtmux serve（或 gtmux tunnel）拿到地址 + token，扫码或手动填入。'
              : 'Run gtmux serve (or gtmux tunnel) on your server for an address + token — scan it or enter it below.'}
          </Text>

          <TouchableOpacity
            testID={TestIds.pairing.scan}
            accessibilityLabel={TestIds.pairing.scan}
            activeOpacity={0.85}
            style={styles.qrBtn}
            onPress={() => {
              setError('');
              setScanning(true);
            }}>
            <Text style={styles.qrText}>◉  {t('scanQR')}</Text>
          </TouchableOpacity>

          <Text style={[styles.or, {color: pal.fg3}]}>—— {t('manualEntry')} ——</Text>

          <Text style={[styles.label, {color: pal.fg2}]}>{t('host')}</Text>
          <TextInput
            testID={TestIds.pairing.host}
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
            accessibilityLabel={TestIds.pairing.connect}
            style={[styles.connect, busy && styles.connectBusy]}
            onPress={connect}
            disabled={busy}>
            {busy ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.connectText}>{t('connect')}</Text>
            )}
          </TouchableOpacity>

          {/* Escape hatch for someone without a Mac handy (e.g. an App Store
              reviewer): a read-only tour with sample data. */}
          {onDemo && (
            <TouchableOpacity onPress={onDemo} style={styles.demoLink}
              accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '没有 Mac？看看演示' : 'No Mac? See a demo'}>
              <Text style={[styles.demoLinkText, {color: pal.fg3}]}>
                {lang === 'zh' ? '没有 Mac？看看演示 →' : 'No Mac handy? See a demo →'}
              </Text>
            </TouchableOpacity>
          )}
        </ScrollView>
      </KeyboardAvoidingView>
      <Modal visible={scanning} animationType="slide" onRequestClose={() => setScanning(false)}>
        <ScanScreen onClose={() => setScanning(false)} onScanned={onScanned} />
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
  demoLink: {alignItems: 'center', paddingVertical: 16},
  demoLinkText: {fontSize: 13},
});
