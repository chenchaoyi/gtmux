// PairingScreen — "Add a Mac". Manual host+token entry (works on the simulator).
// The QR scanner needs react-native-vision-camera + a real device; that's a
// later increment, so the Scan button explains it for now.

import React, {useState} from 'react';
import {
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {GtmuxClient} from '../api/client';
import {useApp} from '../state/AppContext';
import {normalizeHost} from '../pairing/qr';

export function PairingScreen() {
  const {t, pal, pair} = useApp();
  const [host, setHost] = useState('');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const connect = async () => {
    const base = normalizeHost(host);
    if (!base || !token.trim()) {
      setError(t('cantReach'));
      return;
    }
    setBusy(true);
    setError('');
    try {
      const client = new GtmuxClient(base, token.trim());
      const ok = await client.health();
      if (!ok) {
        setError(t('cantReach'));
        return;
      }
      // health passes auth-free; validate the token with a real authed call.
      await client.agents();
      await pair({url: base, token: token.trim(), name: base.replace(/^https?:\/\//, '')});
    } catch (e: any) {
      setError(t('badToken'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={styles.flex}>
        <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
          <Text style={[styles.brand, {color: pal.fg}]}>gtmux</Text>
          <Text style={[styles.title, {color: pal.fg}]}>{t('addMac')}</Text>

          <TouchableOpacity
            style={[styles.qrBtn, {borderColor: pal.divider, backgroundColor: pal.surface}]}
            onPress={() =>
              Alert.alert('gtmux', t('pushDevice'))
            }>
            <Text style={[styles.qrText, {color: pal.fg2}]}>▦  {t('scanQR')}</Text>
          </TouchableOpacity>

          <Text style={[styles.or, {color: pal.fg3}]}>—— {t('manualEntry')} ——</Text>

          <Text style={[styles.label, {color: pal.fg2}]}>{t('host')}</Text>
          <TextInput
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
            value={token}
            onChangeText={setToken}
            placeholder="serve-token"
            placeholderTextColor={pal.fg3}
            autoCapitalize="none"
            autoCorrect={false}
            secureTextEntry
            style={[styles.input, {color: pal.fg, borderColor: pal.divider, backgroundColor: pal.surface}]}
          />

          {!!error && <Text style={styles.error}>{error}</Text>}

          <TouchableOpacity
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
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {flex: 1},
  flex: {flex: 1},
  container: {padding: 24, paddingTop: 48},
  brand: {fontSize: 15, fontWeight: '700', opacity: 0.6, marginBottom: 4},
  title: {fontSize: 28, fontWeight: '700', marginBottom: 28},
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
