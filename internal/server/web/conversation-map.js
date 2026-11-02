/* One loaded-conversation navigator for focused chat and workbench tiles. */
(function () {
  'use strict';
  window.GtmuxConversationMap = {mount: function (main, scroll, turns, T) {
    if(!window.GtmuxTrace || !window.GtmuxTrace.conversationOutline) return function () {};
    var outline = window.GtmuxTrace.conversationOutline(turns,T('en','zh')), nodes = Array.from(scroll.querySelectorAll('.cturn'));
    if (outline.length < 8) return function () {};
    var nav=document.createElement('div');nav.className='conversation-map';
    var track=document.createElement('div');track.className='conversation-map-track';track.tabIndex=0;track.setAttribute('role','slider');track.setAttribute('aria-orientation','vertical');track.setAttribute('aria-label',T('Conversation overview — loaded turns','对话总览 — 已加载轮次'));track.setAttribute('aria-valuemin','1');track.setAttribute('aria-valuemax',String(outline.length));
    var tip=document.createElement('div');tip.className='conversation-map-preview';tip.hidden=true;
    var meta=document.createElement('div'),prompt=document.createElement('strong'),summary=document.createElement('p'),jump=document.createElement('button');meta.className='conversation-map-meta';jump.type='button';jump.textContent=T('Go to turn','跳到这一轮');tip.append(meta,prompt,summary,jump);
    nav.append(track,tip);main.appendChild(nav);main.classList.add('has-conversation-map');
    var selected=0,current=0,touch=false,keyboardMode=false,hideTimer=0,frame=0,positions=[],marks=[];
    var samples=Math.min(outline.length,120);
    for(var i=0;i<samples;i++){var mark=document.createElement('i');mark.setAttribute('aria-hidden','true');mark.dataset.index=String(Math.round(i*(outline.length-1)/(samples-1)));mark.style.top=(i*100/(samples-1))+'%';track.appendChild(mark);marks.push(mark);}
    function name(item){return T('Turn ','第 ')+item.turn+T('',' 轮');}
    function paint(){marks.forEach(function(mark){var index=Number(mark.dataset.index),distance=Math.abs(index-selected),zoom=tip.hidden?0:Math.max(0,1-distance/Math.max(3,outline.length/30));mark.style.setProperty('--map-lens',String(zoom));mark.classList.toggle('current',Math.abs(index-current)<Math.max(1,outline.length/samples/2));});track.setAttribute('aria-valuenow',String(selected+1));track.setAttribute('aria-valuetext',name(outline[selected])+' · '+outline[selected].prompt);}
    function preview(index){clearTimeout(hideTimer);selected=Math.max(0,Math.min(outline.length-1,index));var item=outline[selected];meta.textContent=[item.agent,name(item),T(outline.length+' loaded turns', '已加载 '+outline.length+' 轮')].filter(Boolean).join(' · ');prompt.textContent=item.prompt||T('No message text','无消息正文');summary.textContent=item.summary;tip.hidden=false;tip.style.top=Math.max(8,Math.min(main.clientHeight-200,track.offsetTop+selected/Math.max(1,outline.length-1)*track.clientHeight-65))+'px';paint();}
    function hide(){tip.hidden=true;selected=current;paint();}
    function go(){var node=nodes[outline[selected].sourceIndex];if(!node)return;scroll.scrollTop=scroll.scrollTop+node.getBoundingClientRect().top-scroll.getBoundingClientRect().top-14;current=selected;if(keyboardMode){node.tabIndex=-1;node.focus({preventScroll:true});}node.classList.add('conversation-map-target');setTimeout(function(){node.classList.remove('conversation-map-target');},1200);hide();}
    function measure(){tip.style.width=Math.min(320,Math.max(120,main.clientWidth-48))+'px';positions=outline.map(function(item){var node=nodes[item.sourceIndex];return node?node.getBoundingClientRect().top-scroll.getBoundingClientRect().top+scroll.scrollTop:0;});update();}
    function update(){cancelAnimationFrame(frame);frame=requestAnimationFrame(function(){if(!main.isConnected)return;var target=scroll.scrollTop+scroll.clientHeight*.25,lo=0,hi=positions.length;while(lo<hi){var mid=(lo+hi)>>1;if(positions[mid]<=target)lo=mid+1;else hi=mid;}current=Math.max(0,lo-1);if(tip.hidden)selected=current;paint();});}
    track.onpointermove=function(e){if(keyboardMode && !e.movementX && !e.movementY)return;keyboardMode=false;if(e.pointerType!=='touch')preview(Math.round((e.clientY-track.getBoundingClientRect().top)/track.clientHeight*(outline.length-1)));};
    track.onpointerdown=function(e){touch=e.pointerType==='touch';if(touch)preview(Math.round((e.clientY-track.getBoundingClientRect().top)/track.clientHeight*(outline.length-1)));};
    track.onclick=function(){if(!touch)go();};jump.onclick=go;
    track.onfocus=function(){if(tip.hidden)preview(current);};
    track.onkeydown=function(e){keyboardMode=true;var value=selected;if(e.key==='ArrowUp')value--;else if(e.key==='ArrowDown')value++;else if(e.key==='Home')value=0;else if(e.key==='End')value=outline.length-1;else if(e.key==='Enter'||e.key===' '){e.preventDefault();go();return;}else if(e.key==='Escape'){hide();return;}else return;e.preventDefault();preview(value);};
    nav.onpointerleave=function(e){if(touch || e.pointerType==='touch' || nav.contains(e.relatedTarget))return;hideTimer=setTimeout(hide,200);};nav.onpointerenter=function(){clearTimeout(hideTimer);};nav.onfocusout=function(e){if(!nav.contains(e.relatedTarget))hide();};
    function outside(e){if(!nav.contains(e.target))hide();}
    document.addEventListener('pointerdown',outside);
    scroll.addEventListener('scroll',update,{passive:true});
    var resize=typeof ResizeObserver==='undefined'?null:new ResizeObserver(measure);if(resize){resize.observe(scroll);if(scroll.firstElementChild)resize.observe(scroll.firstElementChild);}
    measure();
    return function(){clearTimeout(hideTimer);cancelAnimationFrame(frame);if(resize)resize.disconnect();document.removeEventListener('pointerdown',outside);scroll.removeEventListener('scroll',update);nav.remove();main.classList.remove('has-conversation-map');};
  }};
})();
