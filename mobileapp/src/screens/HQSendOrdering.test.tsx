import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {SendFailedBar} from '../ui/SendFailedBar';
import {SendDeadlineError} from '../api/sendDeadline';
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

test.each(['reject','refuse','success'])('MSD-008 old HQ %s cannot replace a newer accepted message',async outcome=>{
 let finish!:(r:any)=>void,fail!:(e:Error)=>void;
 mockClient.sendResult.mockReturnValueOnce(new Promise((resolve,reject)=>{finish=resolve;fail=reject;})).mockResolvedValue({ok:true,delivery:'accepted'});
 await mount();await send('first');await send('second');
 await act(async()=>{if(outcome==='reject')fail(new SendDeadlineError());else finish(outcome==='refuse'?{ok:false,reason:'refused'}:{ok:true,delivery:'queued'});});await flush();
 expect(chat()).toMatchObject({pendingPrompt:'second',pendingDelivered:true,pendingDelivery:'accepted'});
 expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
});
test('MSD-008 HQ old reply and old retry cannot cross a session change',async()=>{
 let fail!:(e:Error)=>void;mockClient.sendResult.mockReturnValueOnce(new Promise((_,reject)=>{fail=reject;}));
 await mount();await send('old target');const next={...hq,session_id:'session-B'};mockContext.agents=[next];
 await act(async()=>{tree.update(<HQConsoleView agent={next} workspaceMode />);});await flush();
 await act(async()=>{fail(new SendDeadlineError());});await flush();
 expect(chat().pendingPrompt).toBeUndefined();expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
 await send('new target');expect(chat().pendingPrompt).toBe('new target');
});
test.each(['legacy','native'])('MSD-008 a later %s key does not leave the current HQ text Sending',async route=>{
 let accept!:(r:any)=>void;mockClient.sendResult.mockReturnValueOnce(new Promise(resolve=>{accept=resolve;})).mockResolvedValue({ok:true});
 await mount();await send('text');await act(async()=>{tree.root.findByType(ComposerFn).props[route==='legacy'?'onSend':'onSendKey']({key:'Enter'});});await flush();
 await act(async()=>{accept({ok:true,delivery:'accepted'});});await flush();
 expect(chat()).toMatchObject({pendingPrompt:'text',pendingDelivered:true,pendingDelivery:'accepted'});
});
