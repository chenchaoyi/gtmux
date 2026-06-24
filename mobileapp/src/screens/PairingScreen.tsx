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
import {normalizeHost, parsePairingQR} from '../pairing/qr';
import {BrandMark} from '../ui/BrandMark';
import {ScanScreen} from './ScanScreen';
import {TestIds} from '../constants/testIds';

// onCancel, when provided, renders a Cancel control — set when PairingScreen is
// presented as the "Add a Mac" sheet from ServersScreen (vs. the bare first run).
export function PairingScreen({onCancel}: {onCancel?: () => void} = {}) {
  const {t, pal, pair, lang} = useApp();
  const [host, setHost] = useState('');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [scanning, setScanning] = useState(false);

  // connectWith validates reachability + token, then pairs. Shared by manual
  // entry and the QR scanner.
  const connectWith = async (base: string, tok: string, name: string) => {
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
      await client.agents(); // validate the token with a real authed call
      await pair({url: base, token: tok, name});
    } catch {
      setError(t('badToken'));
    } finally {
      setBusy(false);
    }
  };

  const connect = () => {
    const base = normalizeHost(host);
    connectWith(base, token.trim(), base.replace(/^https?:\/\//, ''));
  };

  const onScanned = (raw: string) => {
    setScanning(false);
    try {
      const m = parsePairingQR(raw);
      connectWith(m.url, m.token, m.name);
    } catch (e: any) {
      setError(e?.message || t('badToken'));
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
            style={[styles.qrBtn, {borderColor: pal.divider, backgroundColor: pal.surface}]}
            onPress={() => {
              setError('');
              setScanning(true);
            }}>
            <Text style={[styles.qrText, {color: pal.fg2}]}>▦  {t('scanQR')}</Text>
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
  qrBtn: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: 'center',
  },
  qrText: {fontSize: 16, fontWeight: '600'},
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
});
