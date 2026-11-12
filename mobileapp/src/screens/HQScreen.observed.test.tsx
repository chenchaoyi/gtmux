import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {HQView} from './HQScreen';
import {ChatView} from '../ui/ChatView';
import {Composer} from '../ui/Composer';
import {Agent} from '../api/types';
jest.mock('../state/AgentsContext',()=>({useAgents:()=>mockContext}));
jest.mock('../state/AppContext',()=>({useApp:()=>({pal:require('../ui/theme').paletteFor('dark'),lang:'en'})}));
jest.mock('../state/WorkspaceContext',()=>({useWorkspace:()=>({select:jest.fn()})}));
jest.mock('../ui/ChatView',()=>({ChatView:()=>null}));
jest.mock('../ui/Composer',()=>({Composer:()=>null}));
jest.mock('./HQHeader',()=>({HQHeader:()=>null}));
const agent={pane_id:'%fixture',agent:'Codex',status:'idle',role:'supervisor'} as Agent;
const mockClient={base:'https://synthetic.invalid',token:'fixture',tasks:jest.fn(async()=>[]),digest:jest.fn(async()=>[]),usage:jest.fn(async()=>null),hqBoard:jest.fn(async()=>({exists:false})),hqKnowledge:jest.fn(async()=>({entries:[],topics:[],promotions:{pending:0},candidates:{pending:0}})),hqEvents:jest.fn(async()=>[]),pane:jest.fn(async()=>({text:''})),transcript:jest.fn(),sendResult:jest.fn(async()=>({ok:true}))};
const mockContext={client:mockClient,agents:[agent],conn:'connected',demo:false};

test('SID006 mounted HQ cannot send from last-known history and resets on verified response',async()=>{
 jest.useFakeTimers();let tree:renderer.ReactTestRenderer|undefined;const original=globalThis.fetch;globalThis.fetch=jest.fn(()=>{throw new Error('no real HTTP');}) as typeof fetch;
 try{
  mockClient.transcript.mockResolvedValue({turns:[],dropped:0,readOnlyHistory:true});
  await act(async()=>{tree=renderer.create(<HQView agent={agent} layout="compact"/>);});
  const chat=()=>tree!.root.findByType(ChatView).props;const composer=()=>tree!.root.findByType(Composer).props;
  expect(chat().readOnlyHistory).toBe(true);expect(composer().enabled).toBe(false);
  await act(async()=>composer().onSend({text:'blocked'}));expect(mockClient.sendResult).not.toHaveBeenCalled();
  mockClient.transcript.mockResolvedValue({turns:[],dropped:0,readOnlyHistory:false,unchanged:true});
  await act(async()=>chat().onRetry());expect(composer().enabled).toBe(true);
 }finally{act(()=>tree?.unmount());jest.clearAllTimers();jest.useRealTimers();globalThis.fetch=original;}
});
