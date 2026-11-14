import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));
const mockClient = {
 base: 'https://synthetic.invalid', tasks: jest.fn(async () => []), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
 hqBoard: jest.fn(async () => ({exists:false})),hqKnowledge:jest.fn(async()=>({entries:[],topics:[],promotions:{pending:0},candidates:{pending:0}})),
 hqEvents:jest.fn(async()=>[]),pane:jest.fn(async()=>({text:''})),transcript:jest.fn(async()=>({turns:[] as any[]})),
 send:jest.fn(async()=>({text:''})),sendResult:jest.fn(),
};
const hq:any={pane_id:'%synthetic',session_id:'session-A',home_id:'home',agent:'Codex',status:'idle',role:'supervisor'};
const mockContext={client:mockClient,agents:[hq],conn:'connected',demo:false};
const ComposerFn=(Composer as unknown as {type:React.ComponentType<any>}).type;
let tree:renderer.ReactTestRenderer;
const flush=async()=>{for(let i=0;i<6;i++)await act(async()=>{await Promise.resolve();});};
async function mount(agent=hq){await act(async()=>{tree=renderer.create(<HQConsoleView agent={agent} workspaceMode />);});await flush();}
async function send(text:string){await act(async()=>{tree.root.findByType(ComposerFn).props.onSend({text,enter:true});});await flush();}
const chat=()=>tree.root.findByType(ChatView).props;
beforeEach(()=>{jest.useFakeTimers();jest.clearAllMocks();mockContext.agents=[hq];mockClient.sendResult.mockReset().mockResolvedValue({ok:true,delivery:'accepted'});mockClient.transcript.mockResolvedValue({turns:[]});});
afterEach(()=>{act(()=>tree?.unmount());jest.useRealTimers();});


test('CRX-002 HQ reload is pending immediately and late superseded results cannot settle it',async()=>{
 await mount();let release!:(v:any)=>void;const held=new Promise(r=>{release=r;});
 mockClient.transcript.mockReturnValueOnce(held as any);
 act(()=>chat().onRetry());expect(chat().retrying).toBe(true);await flush();
 expect((mockClient.transcript.mock.calls as any[][]).at(-1)![1]).toBeUndefined();
 await act(async()=>release({turns:[{prompt:'fresh',response:'loaded'}]}));await flush();
 expect(chat().retrying).toBe(false);expect(chat().turns[0].response).toBe('loaded');
});
test('CRX-004 a send receipt invalidates the pre-send read and refreshes once without waiting for poll interval',async()=>{
 await mount();const count=mockClient.transcript.mock.calls.length;
 mockClient.transcript.mockResolvedValue({turns:[{prompt:'new prompt',time:'now',response:'answer'}]});
 await send('new prompt');expect(mockClient.transcript.mock.calls.length).toBeGreaterThan(count);
 expect(chat().pendingPrompt).toBeUndefined();expect(chat().retrying).toBe(false);
 expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});
