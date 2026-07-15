// AgentAvatar — the agent's OFFICIAL tool icon (fetched from the Mac's installed
// app via /api/icon, like the menu-bar app), falling back to a neutral monogram
// mark when there's no icon hint or the fetch 404s. We do NOT bundle third-party
// logos (DESIGN §6); color is never used for identity.
//
// Colors are passed in explicitly so it's safe on ANY surface — including the
// always-dark chat surface, where the theme palette would be near-black/invisible
// (see the dark-surface trap). Shared by the radar row and the chat view.

import React, {useState} from 'react';
import {Image, StyleSheet, Text, View} from 'react-native';
import {Agent} from '../api/types';
import {agentMark} from './agentMark';
import {useAgentsOptional} from '../state/AgentsContext';

export function AgentAvatar({
  agent,
  size,
  radius,
  bg,
  fg,
  border,
}: {
  agent: Agent;
  size: number;
  radius: number;
  bg: string;
  fg: string;
  border?: string; // omit for no border (e.g. the dark chat surface)
}) {
  // Optional: the Demo screen renders this OUTSIDE an AgentsProvider. No client →
  // no icon fetch → the neutral monogram fallback (which is what Demo wants anyway).
  const client = useAgentsOptional()?.client ?? null;
  const [failed, setFailed] = useState(false);
  const source = !failed && agent.icon && client ? client.iconUri(agent.agent) : null;
  return (
    <View
      style={[
        styles.wrap,
        {
          width: size,
          height: size,
          borderRadius: radius,
          backgroundColor: bg,
          borderWidth: border ? StyleSheet.hairlineWidth : 0,
          borderColor: border,
        },
      ]}>
      {source ? (
        <Image source={source} style={styles.img} resizeMode="contain" onError={() => setFailed(true)} />
      ) : (
        <Text style={[styles.mono, {color: fg, fontSize: Math.round(size * 0.42)}]}>{agentMark(agent.agent)}</Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {alignItems: 'center', justifyContent: 'center', overflow: 'hidden'},
  img: {width: '100%', height: '100%'},
  mono: {fontWeight: '700'},
});
