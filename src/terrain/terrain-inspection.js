import {biomeName,biomeNames,categoricalBiome,coarseMaterial,materialSeed} from './biome-material.js';
import {riverWidthPixels} from './river-width.js';

export function sampleTerrainTile(tile,point,detail){
 const u=Math.max(0,Math.min(32,point.x/tile.step-tile.x*32)),v=Math.max(0,Math.min(32,point.y/tile.step-tile.y*32));
 const x=Math.min(31,Math.floor(u)),y=Math.min(31,Math.floor(v)),a=u-x,b=v-y;
 const points=[tile.points[y*33+x],tile.points[y*33+x+1],tile.points[(y+1)*33+x],tile.points[(y+1)*33+x+1]],weights=[(1-a)*(1-b),a*(1-b),(1-a)*b,a*b];
 const mix=key=>points.reduce((sum,p,k)=>sum+(p[key]??0)*weights[k],0);
 const t=Math.max(0,Math.min(1,detail-tile.level+1)),blend=tile.level===0?1:t*t*(3-2*t);
 const elevation=mix('parent')+(mix('elevation')-mix('parent'))*blend;
 let coverage=0,body=0,best=0;for(let k=0;k<4;k++)if(points[k].waterBody>0){coverage+=weights[k];if(weights[k]>best){best=weights[k];body=points[k].waterBody;}}
 const wet=coverage>=.5,river=tile.level>=3&&mix('riverDepth')>.3,waterLevel=mix(wet?'waterLevel':'riverLevel');
 const biome=points[(b>=.5?2:0)+(a>=.5?1:0)].biome;
 return {...point,...(biome?{biome}:{}),...(points.some(p=>p.desert!==undefined)?{desert:mix('desert')}:{}),...Object.fromEntries(['temperature','moisture','vegetation','rock','sand','snow','wetland','riverDepth'].map(key=>[key,mix(key)])),level:tile.level,elevation,waterBody:wet?body:0,waterLevel,waterDepth:wet||river?Math.max(0,waterLevel-elevation):0};
}

export function renderedRiverAt(features,point,scale,detail){
 const seen=new Set();
 for(const f of features){
  if(f.kind!=='river'||seen.has(f.id)||Math.max(f.level||0,f.morphologyLevel||0)>=detail+1)continue;seen.add(f.id);
  for(let k=1;k<f.path.length;k++){
   const a=f.path[k-1],b=f.path[k],dx=b[0]-a[0],dy=b[1]-a[1],length=dx*dx+dy*dy;
   const t=length?Math.max(0,Math.min(1,((point.x-a[0])*dx+(point.y-a[1])*dy)/length)):0;
   const radius=(riverWidthPixels(f,k-1,scale,detail)*(1-t)+riverWidthPixels(f,k,scale,detail)*t)/2;
   if(Math.hypot(point.x-a[0]-t*dx,point.y-a[1]-t*dy)*scale<=radius)return true;
  }
 }return false;
}

export function geographicLocation(solver,point){
 const g=solver?.environment?.geography;if(!g)return null;
 const latitude=g.latitude_north-(point.y+.5)/solver.H*(g.latitude_north-g.latitude_south);
 const longitude=g.longitude_west+(point.x+.5)/solver.W*(g.longitude_east-g.longitude_west);
 const radians=Math.PI/180,radius=g.planetary_radius;
 return {latitude,longitude,xKm:(longitude-g.longitude_west)*radians*radius*Math.cos(latitude*radians),yKm:(g.latitude_north-latitude)*radians*radius};
}
export function formatMapLocation(solver,point){
 const location=geographicLocation(solver,point);
 if(!location)return `${point.x.toFixed(2)}, ${point.y.toFixed(2)} cells (unscaled map)`;
 const {latitude,longitude,xKm,yKm}=location;
 return `${Math.abs(latitude).toFixed(2)}\u00b0${latitude>=0?'N':'S'}, ${Math.abs(longitude).toFixed(2)}\u00b0${longitude>=0?'E':'W'} \u00b7 ${xKm.toFixed(1)} km east of west edge, ${yKm.toFixed(1)} km south of north edge`;
}

// The readout uses the same material classifier and sampled fields as the
// visible terrain, not the older WFC palette assigned to its nearest cell.
export function terrainInspection(solver,point,sample=null){
 if(!solver?.config?.types||!Number.isFinite(solver.W)||!Number.isFinite(solver.H)||!point)return null;
 const x=Math.max(0,Math.min(solver.W-1,point.x)),y=Math.max(0,Math.min(solver.H-1,point.y));
 const cell=Math.round(y)*solver.W+Math.round(x),type=solver.config.types[solver.typeAt(cell)];
 const env=solver.environment;
 if(!env||solver.pinned?.[cell])return {terrain:type?.name||'Unresolved terrain',point:{x,y}};
 if(!sample){
  const left=Math.floor(x),top=Math.floor(y),a=x-left,b=y-top;
  const nodes=[[left,top,(1-a)*(1-b)],[Math.min(left+1,solver.W-1),top,a*(1-b)],[left,Math.min(top+1,solver.H-1),(1-a)*b],[Math.min(left+1,solver.W-1),Math.min(top+1,solver.H-1),a*b]];
  sample={};let wet=0,best=0,body=0;
  for(const [xx,yy,weight] of nodes){const i=yy*solver.W+xx;for(const [key,value] of Object.entries(coarseMaterial(env,i)))if(key!=='biome')sample[key]=(sample[key]||0)+value*weight;
   sample.elevation=(sample.elevation||0)+(env.fields.elevation?.[i]||0)*weight;
   const water=env.fields.waterBody?.[i]||0;if(water){wet+=weight;if(weight>best){best=weight;body=water;}}
  }sample.waterBody=wet>=.5?body:0;
 }
 sample={...sample,...(env.fields.biome?{biome:categoricalBiome(env,cell)}:{})};
 let material=biomeName(sample,{x,y,seed:materialSeed(env.options?.seed||0)}),terrain;
 if(sample.waterBody>0){material='water';terrain=sample.waterBody===1?'Ocean / sea':'Lake';}
 else if(sample.riverVisible||(sample.level>=3&&sample.riverDepth>.3)){material='river';terrain='River';}
 else terrain=sample.biome?biomeNames[material]:({forest:'Forest',mountain:'Mountain terrain',sand:'Sand / shore',swamp:'Wetland'}[material]||biomeNames[material]);
 return {terrain,material,point:{x,y},sample};
}
