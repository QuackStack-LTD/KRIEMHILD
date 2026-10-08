import {MapCamera,detailAtScale,detailName,smooth} from './map-camera.js';
import {DetailTiles} from './detail-tiles.js';
import {makeDetailPalette,tileImage,drawDetailFeatures,drawDetailOverlay} from './detail-render.js';

export function createExplorer(container,source,brushOn){
  const canvas=document.createElement('canvas');canvas.id='detailMap';canvas.className='detail-map';canvas.tabIndex=0;canvas.setAttribute('aria-label','Explore terrain: scroll to zoom at cursor, drag to pan, plus or minus to zoom, zero to fit');container.append(canvas);
  const ctx=canvas.getContext('2d'),camera=new MapCamera(),abort=new AbortController();
  let solver=null,tiles=null,palette=null,layer='terrain',active=false,frame=0,drag=null,width=0,height=0,disposed=false,images=new WeakMap();
  const status=document.getElementById('detailStatus');
  function listen(el,event,fn,opts={}){el.addEventListener(event,fn,{...opts,signal:abort.signal});}
  function queue(force=false){if(force&&frame){cancelAnimationFrame(frame);frame=0;}if(!frame&&!disposed)frame=requestAnimationFrame(render);}
  function fit(){if(!solver)return;camera.fit(width,height,solver.W-1,solver.H-1);queue(true);}
  function resize(){const oldW=width,oldH=height;const center=camera.point(oldW/2,oldH/2);width=container.clientWidth;height=container.clientHeight;const dpr=Math.min(2,devicePixelRatio||1);canvas.width=Math.round(width*dpr);canvas.height=Math.round(height*dpr);if(solver){camera.minimum=Math.min(width/(solver.W-1),height/(solver.H-1));if(!oldW||camera.scale<=camera.minimum)fit();else{camera.x=width/2-center.x*camera.scale;camera.y=height/2-center.y*camera.scale;}}queue(true);}
  const observer=new ResizeObserver(resize);observer.observe(container);resize();
  function local(e){const r=canvas.getBoundingClientRect();return {x:e.clientX-r.left,y:e.clientY-r.top};}
  function zoom(x,y,factor){camera.zoomAt(x,y,factor);queue(true);}
  listen(canvas,'wheel',e=>{e.preventDefault();const p=local(e),delta=e.deltaY*(e.deltaMode===1?16:e.deltaMode===2?height:1);zoom(p.x,p.y,Math.exp(-delta*.0015));},{passive:false});
  listen(canvas,'pointerdown',e=>{canvas.focus({preventScroll:true});if(brushOn())return;drag={id:e.pointerId,x:e.clientX,y:e.clientY};canvas.setPointerCapture(e.pointerId);canvas.classList.add('dragging');});
  listen(canvas,'pointermove',e=>{if(!drag)return;camera.x+=e.clientX-drag.x;camera.y+=e.clientY-drag.y;drag.x=e.clientX;drag.y=e.clientY;queue(true);});
  const release=()=>{drag=null;canvas.classList.remove('dragging');};listen(canvas,'pointerup',release);listen(canvas,'pointercancel',release);
  for(const name of ['mousemove','mouseleave','mousedown','mouseup','contextmenu'])listen(canvas,name,e=>{if(name==='contextmenu')e.preventDefault();if(name==='mousedown'&&!brushOn())return;source.dispatchEvent(new MouseEvent(name,{clientX:e.clientX,clientY:e.clientY,button:e.button,buttons:e.buttons,bubbles:false,cancelable:true}));});
  listen(canvas,'keydown',e=>{if(['+','=','-','0'].includes(e.key)){e.preventDefault();e.stopPropagation();if(e.key==='0')fit();else zoom(width/2,height/2,e.key==='-'?.5:2);}});
  listen(document.getElementById('zoomIn'),'click',()=>zoom(width/2,height/2,2));listen(document.getElementById('zoomOut'),'click',()=>zoom(width/2,height/2,.5));listen(document.getElementById('fitMap'),'click',fit);
  function render(){
    frame=0;if(!active||!solver||!width||!height)return;
    const bounds=camera.bounds(width,height),detail=detailAtScale(camera.scale),visible=tiles.request(bounds,camera.scale);
    ctx.setTransform(canvas.width/width,0,0,canvas.height/height,0,0);ctx.clearRect(0,0,width,height);ctx.fillStyle='#111418';ctx.fillRect(0,0,width,height);ctx.imageSmoothingEnabled=true;
    // Clip once to the finite world; border tiles contain replicated edge nodes.
    ctx.save();ctx.beginPath();ctx.rect(camera.x,camera.y,(solver.W-1)*camera.scale,(solver.H-1)*camera.scale);ctx.clip();
    ctx.drawImage(source,camera.x,camera.y,(solver.W-1)*camera.scale,(solver.H-1)*camera.scale);
    let fading=false;
    for(const item of visible){const {tile,alpha,arrival}=item;let entry=images.get(tile);const morph=Math.round(alpha*64)/64;
      if(!entry||entry.morph!==morph){entry={morph,image:tileImage(tile,solver,palette,layer,morph)};images.set(tile,entry);}
      const span=32*tile.step;ctx.globalAlpha=tile.level===0?1:alpha*arrival;
      ctx.drawImage(entry.image,.5,.5,32,32,camera.x+tile.x*span*camera.scale,camera.y+tile.y*span*camera.scale,span*camera.scale,span*camera.scale);
      if(arrival<1)fading=true;
    }
    ctx.globalAlpha=1;drawDetailFeatures(ctx,visible,camera,detail,layer);drawDetailOverlay(ctx,solver,camera,layer,width,height);ctx.restore();
    status.textContent=`${detailName(detail)} · ${(camera.scale/camera.minimum).toFixed(1)}×${tiles.pending.size?' · refining…':''}${tiles.error?' · '+tiles.error:''}`;
    canvas.dataset.detail=String(detail);canvas.dataset.scale=String(camera.scale);canvas.dataset.offsetX=String(camera.x);canvas.dataset.offsetY=String(camera.y);canvas.dataset.tiles=String(tiles.cache.size);
    if(fading)queue();
  }
  return {
    setWorld(next,nextLayer='terrain',enabled=true){
      const changed=next?.id!==solver?.id;solver=next;
      if(width!==container.clientWidth||height!==container.clientHeight)resize();
      active=Boolean(enabled&&solver?.environment);canvas.hidden=!active;container.classList.toggle('exploring',active);
      if(changed){tiles?.dispose();tiles=solver?.environment?new DetailTiles(solver,queue):null;images=new WeakMap();fit();}
      if(!active)tiles?.pause();
      if(active){palette=makeDetailPalette(solver,nextLayer);images=new WeakMap();layer=nextLayer;queue(true);}
      document.getElementById('mapNavigation').hidden=!solver?.environment;
      for(const id of ['zoomIn','zoomOut','fitMap'])document.getElementById(id).hidden=!active;
      document.querySelector('.map-navigation span').textContent=active?'Scroll at cursor to explore ? drag to pan':'Scroll at cursor ? drag to orbit ? right-drag to pan';
      if(width!==container.clientWidth||height!==container.clientHeight)resize();
    },
    point(e){if(!active)return null;const p=local(e);return camera.point(p.x,p.y);},
    sample(e){
      if(!active||!tiles)return null;const cursor=local(e),p=camera.point(cursor.x,cursor.y);
      if(p.x<0||p.y<0||p.x>solver.W-1||p.y>solver.H-1)return null;
      const detail=detailAtScale(camera.scale);
      for(let level=Math.ceil(detail);level>=0;level--){
        const span=32/2**level,t=tiles.cache.get(`${level}/${Math.floor(p.x/span)}/${Math.floor(p.y/span)}`);if(!t)continue;
        const u=(p.x-t.x*span)/t.step,v=(p.y-t.y*span)/t.step,x=Math.min(31,Math.floor(u)),y=Math.min(31,Math.floor(v)),a=u-x,b=v-y;
        const points=[t.points[y*33+x],t.points[y*33+x+1],t.points[(y+1)*33+x],t.points[(y+1)*33+x+1]],weights=[(1-a)*(1-b),a*(1-b),(1-a)*b,a*b];
        const mix=key=>points.reduce((sum,value,i)=>sum+(value[key]??0)*weights[i],0),blend=level===0?1:smooth(detail-level+1);
        const elevation=mix('parent')+(mix('elevation')-mix('parent'))*blend,waterLevel=mix('waterLevel'),wet=points.reduce((s,p,i)=>s+(p.waterBody>0?weights[i]:0),0)>.5;
        const river=level>=3&&mix('riverDepth')>0,localWaterLevel=wet?waterLevel:mix('riverLevel');
        return {...p,level,elevation,waterDepth:wet||river?Math.max(0,localWaterLevel-elevation):0,waterLevel:localWaterLevel,temperature:mix('temperature')};
      }
      return null;
    },
    pixelScale(){return active?camera.scale:null;},
    cameraState(){const center=camera.point(width/2,height/2);return {center,scale:camera.scale};},
    restoreCamera(saved){if(!saved?.center||!Number.isFinite(saved.scale)||saved.scale<=0||![saved.center.x,saved.center.y].every(Number.isFinite))return;camera.scale=Math.max(camera.minimum,Math.min(3072,saved.scale));camera.x=width/2-saved.center.x*camera.scale;camera.y=height/2-saved.center.y*camera.scale;queue(true);},
    fit,refresh(){images=new WeakMap();queue();},
    dispose(){disposed=true;abort.abort();observer.disconnect();tiles?.dispose();cancelAnimationFrame(frame);canvas.remove();},
  };
}
