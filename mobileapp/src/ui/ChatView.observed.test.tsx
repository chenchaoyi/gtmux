import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';

test('SID006 last-known history is explicitly read only in both languages',()=>{
 const agent={pane_id:'%fixture',agent:'Codex',status:'idle'} as Agent;
 for (const lang of ['en','zh'] as const) {
  let tree!:renderer.ReactTestRenderer;
  const props={agent,lines:[],status:'idle' as const,fontSize:13,pal:paletteFor('dark'),lang,loading:false,turns:[{prompt:'question',response:'kept reply',time:''}]};
  act(()=>{tree=renderer.create(<ChatView {...props} readOnlyHistory/>);});
  try {
   const banner=tree.root.findByProps({testID:'history-read-only'});
   expect(banner.props.children).toContain(lang==='zh'?'上次关联':'Last-known');
   act(()=>tree.update(<ChatView {...props}/>));
   expect(tree.root.findAllByProps({testID:'history-read-only'})).toHaveLength(0);
  }finally{act(()=>tree.unmount());}
 }
});
