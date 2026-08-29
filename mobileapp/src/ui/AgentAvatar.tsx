// AgentAvatar — the agent's OFFICIAL tool icon (fetched from the Mac's installed
// app via /api/icon, like the menu-bar app), falling back to a neutral monogram
// mark when there's no icon hint or the fetch 404s. We do NOT bundle third-party
// logos (DESIGN §6); color is never used for identity.
//
// Colors are passed in explicitly so it's safe on ANY surface — including the
// always-dark chat surface, where the theme palette would be near-black/invisible
// (see the dark-surface trap). Shared by the radar row and the chat view.

import React, {useEffect, useRef, useState} from 'react';
import {Image, StyleSheet, Text, View} from 'react-native';
import {Agent} from '../api/types';
import {agentMark} from './agentMark';
import {useAgentsOptional} from '../state/AgentsContext';

// A failed icon request must not be a permanent verdict. Every row for an agent asks
// for the SAME icon URI, so one row showing the monogram while its neighbours show the
// icon is never "this agent has no icon" — it is one request that did not land.
// Measured 2026-08-29 over 4G: 14 Claude rows, identical `icon` hints from the Mac, one
// monogram. The latch was per component instance and never reset, so that row stayed
// wrong for as long as it stayed mounted.
//
// Backoff, not a loop: an icon that is genuinely unreachable stops asking after these,
// and the monogram shows throughout the waiting so nothing flickers into a blank box.
const RETRY_MS = [800, 2500, 6000];

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
  const icon = agent.icon && client ? client.iconUri(agent.agent) : null;
  const uri = icon?.uri ?? '';
  const [tries, setTries] = useState(0); // also the <Image> key: a bump re-requests
  const [errored, setErrored] = useState(false); // this attempt failed; the mark stands in
  const [gaveUp, setGaveUp] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // A different icon is a fresh question — it must never inherit the previous one's verdict.
  useEffect(() => {
    setTries(0);
    setErrored(false);
    setGaveUp(false);
  }, [uri]);

  useEffect(
    () => () => {
      if (timer.current) {
        clearTimeout(timer.current);
      }
    },
    [],
  );

  const onError = () => {
    setErrored(true);
    if (tries >= RETRY_MS.length) {
      setGaveUp(true); // out of tries — the icon is unreachable, not slow
      return;
    }
    timer.current = setTimeout(() => {
      setErrored(false);
      setTries(n => n + 1);
    }, RETRY_MS[tries]);
  };

  const source = !gaveUp && !errored ? icon : null;
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
        <Image key={tries} source={source} style={styles.img} resizeMode="contain" onError={onError} />
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
