import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {ParticipantProfileModal} from './ParticipantProfileModal';
import type {Agent} from '../api/types';
import type {GtmuxClient} from '../api/client';

test.each(['codex','claude','opencode'])('CRX-005 %s equal adjacent process displays are grouped, not hidden', async name=>{
 let tree!:renderer.ReactTestRenderer;
 const same={name,path:'/synthetic/'+name};
 const client={participantProfile:jest.fn().mockResolvedValue({agent:name,session_id:'s',historical:false,facts:[],execution_chain:[same,same,{name:'launcher',path:'/synthetic/launcher'},same,{...same,arguments:['different']}],chain_observed_at:1000})} as unknown as GtmuxClient;
 await act(async()=>{tree=renderer.create(<ParticipantProfileModal client={client} target={{kind:'agent',agent:{agent:name,pane_id:'%fixture',session_id:'s'} as Agent}} lang="en" onClose={()=>{}}/>);});
 try{
  await act(async()=>tree.root.findByProps({testID:'profile-chain-toggle'}).props.onPress());
  const texts=tree.root.findAllByType(Text).map(n=>String(n.props.children)).join(' ');
  expect(texts).toContain(name+' ×2');expect(texts).not.toContain('replayable');
  expect(texts).not.toContain('not the processes');
  expect(tree.root.findAllByType(TouchableOpacity).filter(n=>/^profile-copy-/.test(n.props.testID??''))).toHaveLength(4);
 }finally{act(()=>tree.unmount());}
});
