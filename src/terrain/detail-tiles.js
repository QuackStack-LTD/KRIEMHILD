import {detailAtScale,smooth} from './map-camera.js';
import {coarseMaterial} from './biome-material.js';

export const tileKey=(level,x,y)=>`${level}/${x}/${y}`;
export function visibleTiles(bounds,level,width,height){
  const span=32/2**level,result=[];
  const x0=Math.max(0,Math.floor(bounds.x/span)),y0=Math.max(0,Math.floor(bounds.y/span));
  const x1=Math.min(Math.ceil(width/span)-1,Math.floor((bounds.x+bounds.width)/span));
  const y1=Math.min(Math.ceil(height/span)-1,Math.floor((bounds.y+bounds.height)/span));
  for(let y=y0;y<=y1;y++)for(let x=x0;x<=x1;x++)result.push({level,x,y,key:tileKey(level,x,y)});
  return result;
}

// Bounded read-through cache. Aborted/late responses cannot enter a new world.
export class DetailTiles {
  constructor(solver,onChange){
    this.solver=solver;this.onChange=onChange;this.cache=new Map();this.pending=new Map();this.queue=[];this.epoch=0;this.error='';this.retry=0;this.failures=new Map();
    solver.detailStores??=new Set();solver.detailStores.add(this);
    // Original samples are already available: world tiles never wait for a
    // network replacement and are the exact parents of every fetched tile.
    const f=solver.environment.fields;
    for(const t of visibleTiles({x:0,y:0,width:solver.W-1,height:solver.H-1},0,solver.W-1,solver.H-1)){
      const points=[];for(let y=0;y<=32;y++)for(let x=0;x<=32;x++){
        const cell=Math.min(solver.H-1,t.y*32+y)*solver.W+Math.min(solver.W-1,t.x*32+x);
        const gx=cell%solver.W,gy=Math.floor(cell/solver.W),dx=(f.elevation[gy*solver.W+Math.min(solver.W-1,gx+1)]-f.elevation[gy*solver.W+Math.max(0,gx-1)])/2,dy=(f.elevation[Math.min(solver.H-1,gy+1)*solver.W+gx]-f.elevation[Math.max(0,gy-1)*solver.W+gx])/2;
        points.push({...coarseMaterial(solver.environment,cell),gradient:[dx,dy,dx,dy],cell,elevation:f.elevation[cell],parent:f.elevation[cell],waterBody:f.waterBody[cell],waterDepth:f.waterDepth[cell],waterLevel:f.waterLevel[cell],temperature:f.temperature[cell]});
      }
      this.cache.set(t.key,{...t,size:33,step:1,points,features:[],loaded:performance.now()-1000,provisional:true});
    }
  }
  request(bounds,pixelsPerCell){
    this.detail=detailAtScale(pixelsPerCell);const target=Math.ceil(this.detail);
    // Always fetch parents first; holes are filled by the already available world.
    const regions=Array.isArray(bounds)?bounds:[bounds],unique=new Map();for(let level=0;level<=target;level++)for(const region of regions)for(const tile of visibleTiles(region,level,this.solver.W-1,this.solver.H-1))unique.set(tile.key,tile);const wanted=[...unique.values()];
    this.wanted=new Set(wanted.map(t=>t.key));
    for(const [key,controller] of this.pending)if(!this.wanted.has(key)){controller.abort();this.pending.delete(key);}
    this.queue=wanted.filter(t=>(!this.cache.has(t.key)||this.cache.get(t.key).provisional)&&!this.pending.has(t.key)&&(this.failures.get(t.key)?.until??0)<Date.now());this.pump();
    return this.frame(bounds);
  }
  frame(bounds){
    const result=[],seen=new Set(),regions=Array.isArray(bounds)?bounds:[bounds];for(let level=0;level<=Math.ceil(this.detail);level++)for(const region of regions)for(const t of visibleTiles(region,level,this.solver.W-1,this.solver.H-1)){
      if(seen.has(t.key))continue;seen.add(t.key);
      const tile=this.cache.get(t.key);if(!tile)continue;
      // Scale-based geomorph; arrival fade only bridges network latency.
      const alpha=level===0?1:smooth(this.detail-level+1);
      result.push({tile,alpha,arrival:smooth((performance.now()-tile.loaded)/180)});
    }
    return result;
  }
  pump(){
    while(this.pending.size<3&&this.queue.length){
      const task=this.queue.shift(),controller=new AbortController(),epoch=this.epoch;this.pending.set(task.key,controller);
      fetch(`/api/sessions/${this.solver.id}/detail/${task.key}`,{signal:controller.signal}).then(async r=>{const data=await r.json();if(!r.ok){const error=new Error(data.error||`Detail request failed (${r.status})`);error.status=r.status;throw error;}if(r.headers.get('X-World-Unsaved')==='true')this.solver.setSaveState?.({unsaved:true});return data;}).then(tile=>{
        if(epoch!==this.epoch||controller.signal.aborted)return;
        tile.loaded=performance.now();this.cache.set(task.key,tile);this.error='';
        for(const key of this.cache.keys()){if(this.cache.size<=192)break;if(!this.wanted.has(key)&&this.cache.get(key).level>0)this.cache.delete(key);}
      }).catch(error=>{if(error.name!=='AbortError'&&epoch===this.epoch){this.error=error.message;const count=(this.failures.get(task.key)?.count??0)+1;this.failures.set(task.key,{count,until:count>3||error.status===404?Infinity:Date.now()+2000});clearTimeout(this.retry);if(count<=3&&error.status!==404)this.retry=setTimeout(()=>this.onChange(),2100);}}).finally(()=>{
        if(this.pending.get(task.key)===controller)this.pending.delete(task.key);
        if(epoch===this.epoch){this.pump();this.onChange();}
      });
    }
  }
  pause(){this.epoch++;for(const c of this.pending.values())c.abort();this.pending.clear();this.queue=[];this.error='';this.failures.clear();clearTimeout(this.retry);}
  invalidate(bounds){
    // A dirty rectangle includes the normal/parent halo supplied by the server.
    const intersects=t=>!bounds||t.x*32/2**t.level<=bounds.x+bounds.width&&(t.x+1)*32/2**t.level>=bounds.x&&t.y*32/2**t.level<=bounds.y+bounds.height&&(t.y+1)*32/2**t.level>=bounds.y;
    for(const [key,tile]of this.cache)if(intersects(tile)){this.cache.delete(key);this.failures.delete(key);}
    for(const [key,c]of this.pending){const [level,x,y]=key.split('/').map(Number);if(intersects({level,x,y})){c.abort();this.pending.delete(key);}}
    this.onChange();
  }
  async whenIdle(){const end=Date.now()+60000;while(this.pending.size||this.queue.length){if(Date.now()>end)throw new Error('Detail is still loading. Wait for refinement to finish and save again.');await new Promise(resolve=>setTimeout(resolve,50));}if(this.error)throw new Error('Some visible detail did not load: '+this.error);}
  dispose(){this.pause();this.cache.clear();this.solver.detailStores?.delete(this);}
}
