import biomeCatalog from '../../internal/terrain/biomes.json' with {type:'json'};
export {biomeCatalog};
export const biomeNames=Object.freeze(Object.fromEntries(biomeCatalog.map(b=>[b.id,b.name])));
export function categoricalBiome(environment,i){return biomeCatalog[environment?.fields?.biome?.[i]]?.id;}
const clamp=v=>Math.max(0,Math.min(1,v));
const smooth=v=>{v=clamp(v);return v*v*(3-2*v);};
export const naturalMaterials=Object.freeze(Object.fromEntries(biomeCatalog.map(b=>[b.id,b.color.match(/\w{2}/g).map(c=>parseInt(c,16))])));

// World-coordinate value noise produces connected ecotone lobes, never a
// screen-space blur or independently randomized patches at each zoom level.
export function materialNoise(x,y,seed=0){
 const ix=Math.floor(x),iy=Math.floor(y),a=smooth(x-ix),b=smooth(y-iy);
 const hash=(x,y)=>{let h=Math.imul(x,374761393)^Math.imul(y,668265263)^seed;h=Math.imul(h^(h>>>13),1274126177);return ((h^(h>>>16))>>>0)/4294967295;};
 const p=hash(ix,iy),q=hash(ix+1,iy),r=hash(ix,iy+1),s=hash(ix+1,iy+1);
 return (p+(q-p)*a)*(1-b)+(r+(s-r)*a)*b;
}
export function materialSeed(seed=''){let h=2166136261;for(const ch of String(seed))h=Math.imul(h^ch.charCodeAt(0),16777619);return h>>>0;}
export function biomeName(p,position){
 // New worlds carry a categorical cover chosen from the monthly water budget.
 // Texture and relief can vary within it; neither can reclassify its climate.
 if(naturalMaterials[p.biome])return p.biome;
 let name='grass';
 const x=position?.x??0,y=position?.y??0,seed=position?.seed??0;
 const broad=materialNoise(x*.31,y*.31,seed),fine=materialNoise(x*1.7+broad,y*1.7-broad,seed^9137);
 const lobe=(broad-.5)*.8+(fine-.5)*.2;
 const wet=clamp(p.moisture??.5),vegetation=clamp(p.vegetation??wet),rock=clamp(p.rock??0),sand=clamp(p.sand??0),temp=p.temperature??15;
 // One material per location. Geographic lobes shape the boundary, while
 // categorical selection prevents interpolated colors between adjacent biomes.
 if(vegetation-.50+lobe*.22>.03&&temp> -7)name='forest';
 // Dryland extent comes from annual precipitation / evaporation, not noise.
 // Older saved tiles use the same fixed dryness threshold as a fallback.
 if((p.desert!==undefined?p.desert>.5:wet<.25)&&rock<.6)name='desert';
 if(rock-.27+lobe*.17>.11)name='mountain';
 if(2-temp+lobe*5>3.5&&rock<.7)name='tundra';
 if((p.wetland??0)-.48+lobe*.08>.11)name='swamp';
 if((p.snow??0)-.26+lobe*.15>.15&&temp<.5)name='snow';
 if(sand-.2+lobe*.12>.14&&rock<.6)name='sand';
 return name;
}

export function biomeColor(p,colors=naturalMaterials,position){
 const name=biomeName(p,position);return [...(colors[name]||naturalMaterials[name])];
}

export function coarseMaterial(environment,i){
 const f=environment.fields,v=(name,fallback=0)=>f[name]?.[i]??fallback;
 const rock=clamp(v('slope')*1.2+v('mountainCore')*.4);
 const sand=clamp(v('dune')+v('sandSupply')*.15*v('aridity'))*(1-rock);
 return {...(f.biome?{biome:categoricalBiome(environment,i)}:{}),temperature:v('temperature',15),moisture:v('moisture',.5),vegetation:v('moisture',.5)*(1-v('snow')),rock,sand:Math.max(sand,v('beach')),snow:v('snow'),wetland:v('wetland'),...(f.desert?{desert:v('desert')}:{})};
}
