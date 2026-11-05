// DemoScreen — a fully-clickable, no-server tour: the REAL radar + DetailView
// rendered over a fake client (demoClient) through DemoAgentsProvider. Reached only
// via "See a demo" on the pairing screen. Tap any row to open its real detail
// (terminal + chat + composer, all canned); a DEMO chip follows you into detail so
// sample output is never mistaken for a live Mac; a sticky "Pair your Mac" stays on
// screen. Everything resets on exit. Doubles as the App Review path — Apple sanctions
// a fully-featured demo mode with demonstration data in lieu of a demo account.
//
// F7: the fake world is now ALIVE enough to show the core loop — approving the
// hero's permission walks it waiting→working→idle(+latest) on this very radar
// (demoClient pushes fresh agents through setAgents), and the chief-of-staff disc
// (ui/HQDisc, the same floating control the real radar shows) opens the real HQScreen
// over a canned digest + one preset exchange.

import React, {useEffect, useMemo, useState} from 'react';
import {Agent} from '../api/types';
import {useApp} from '../state/AppContext';
import {DemoAgentsProvider} from '../state/AgentsContext';
import {WorkspaceProvider} from '../state/WorkspaceContext';
import {sampleAgents} from '../ui/demoData';
import {makeDemoClient} from '../ui/demoClient';
import {SplitShell} from './SplitShell';
import {useSizeClass} from '../ui/layout';

export function DemoScreen({onExit, onPair}: {onExit: () => void; onPair: () => void}) {
  const {lang} = useApp();
  // The scripted world lives in state so demoClient's status arc re-renders the
  // real radar; a fresh client per Demo session resets everything on reopen.
  const [agents, setAgents] = useState<Agent[]>(() => sampleAgents());
  const client = useMemo(() => makeDemoClient(lang === 'zh' ? 'zh' : 'en', setAgents), [lang]);
  useEffect(() => {
    let active = true;
    client.agents().then(rows => { if (active) setAgents(rows); });
    return () => { active = false; client.dispose(); };
  }, [client]);
  // The demo is the REAL shells over the fake client (SURFACES.md §3): the regular shell
  // (sidebar + main pane) on an iPad-sized window, the phone's radar otherwise. It used
  // to carry its own radar, and fell behind the shipped one twice.
  const sizeClass = useSizeClass();
  const demo = useMemo(() => ({onExit, onPair}), [onExit, onPair]);
  const navigateSel = React.useCallback(() => {}, []);

  return (
    <DemoAgentsProvider client={client} agents={agents}>
      <WorkspaceProvider mode={sizeClass} navigate={navigateSel} inlineSelection>
        <SplitShell demo={demo} />
      </WorkspaceProvider>
    </DemoAgentsProvider>
  );
}
