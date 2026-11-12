import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {ChatView} from '../ui/ChatView';
import {Composer} from '../ui/Composer';

jest.mock('../state/AgentsContext',()=>({useAgents:()=>mockContext,useAgentsOptional:()=>mockContext}));
jest.mock('../state/AppContext',()=>({useApp:()=>({pal:require('../ui/theme').paletteFor('dark'),lang:'en',defaultDetailMode:'chat',fontPref:'auto',returnSends:false,mac:{name:'Fixture'}})}));
jest.mock('../state/WorkspaceContext',()=>({useWorkspace:()=>({select:jest.fn()})}));
jest.mock('../ui/ChatView',()=>({ChatView:()=>null}));
jest.mock('../ui/Composer',()=>({Composer:()=>null}));
jest.mock('./HQHeader',()=>({HQHeader:()=>null}));
let mockContext:any;
function deferred(){let resolve!:(value:any)=>void;const promise=new Promise<any>(r=>{resolve=r;});return {promise,resolve};}
function client(){return {base:'https://fixture.invalid',token:'fixture',pane:jest.fn(),transcript:jest.fn(async()=>({turns:[],dropped:0,readOnlyHistory:true})),sendResult:jest.fn(),panes:jest.fn(async()=>[]),tasks:jest.fn(async()=>[]),options:jest.fn(async()=>[]),theme:jest.fn(async()=>undefined),digest:jest.fn(async()=>[]),usage:jest.fn(async()=>null),hqBoard:jest.fn(async()=>({exists:false})),hqKnowledge:jest.fn(async()=>({entries:[],topics:[],promotions:{pending:0},candidates:{pending:0}})),hqEvents:jest.fn(async()=>[])};}
const flattened=(tree:renderer.ReactTestRenderer)=>tree.root.findByType(ChatView).props.lines.flat().map((n:any)=>n.text).join('\n');

test.each(['Detail','HQ'] as const)('LPV002 %s retires old captures across exact client/pane changes',async surface=>{
 for(const change of ['same-url-new-client','new-pane'] as const){
  jest.useFakeTimers();let tree:renderer.ReactTestRenderer|undefined;
  const a=client(),b=client(),first=deferred(),late=deferred(),next=deferred();
  const originalFetch=globalThis.fetch;globalThis.fetch=jest.fn(()=>{throw new Error('no real HTTP');}) as typeof fetch;
  const agent:any={pane_id:'%fixture-a',session:'fixture',agent:'Codex',source:'tmux',status:'working',role:'supervisor',observed_conversation:{session_id:'history',home_id:'fixture',state:'last-known'}};
  a.pane.mockImplementation((pane:string)=>pane==='%fixture-b'?next.promise:late.promise).mockReturnValueOnce(first.promise);b.pane.mockReturnValue(next.promise);
  mockContext={client:a,agents:[agent],conn:'live',isGuest:false,inputPanes:[],demo:false};
  const view=(current:any)=>surface==='Detail'?<DetailView agent={current} initialMode="chat"/>:<HQConsoleView agent={current} showInspector={false}/>;
  try {
   await act(async()=>{tree=renderer.create(view(agent));});
   await act(async()=>first.resolve({text:'source-A',cols:80}));expect(flattened(tree!)).toContain('source-A');
   await act(async()=>jest.advanceTimersByTime(1500));expect(a.pane.mock.calls.length).toBeGreaterThanOrEqual(2);
   const current=change==='new-pane'?{...agent,pane_id:'%fixture-b'}:agent;
   mockContext={...mockContext,client:change==='new-pane'?a:b,agents:[current]};
   await act(async()=>tree!.update(view(current)));
   expect(flattened(tree!)).not.toContain('source-A');
   await act(async()=>late.resolve({text:'late-source-A',cols:80}));expect(flattened(tree!)).not.toContain('source-A');
   await act(async()=>next.resolve({text:'current-source-B',cols:80}));expect(flattened(tree!)).toContain('current-source-B');
   const composer=tree!.root.findByType(Composer).props;expect(composer.enabled).toBe(false);expect(composer.draftOnly).toBe(true);
   await act(async()=>composer.onSend({text:'must not send'}));expect(a.sendResult).not.toHaveBeenCalled();expect(b.sendResult).not.toHaveBeenCalled();
  }finally{act(()=>tree?.unmount());late.resolve({text:''});next.resolve({text:''});jest.clearAllTimers();jest.useRealTimers();globalThis.fetch=originalFetch;}
 }
});
