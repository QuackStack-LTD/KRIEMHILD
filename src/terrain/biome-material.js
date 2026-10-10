const clamp=v=>Math.max(0,Math.min(1,v));
const smooth=v=>{v=clamp(v);return v*v*(3-2*v);};
export const naturalMaterials=Object.freeze({grass:[113,148,78],forest:[53,103,59],desert:[158,141,99],mountain:[123,119,109],tundra:[143,150,127],snow:[224,232,231],sand:[191,172,124],swamp:[76,115,103]});

// World-coordinate value noise produces connected ecotone lobes, never a
// screen-space blur or independently randomized patches at each zoom level.
export function materialNoise(x,y,seed=0){
 const ix=Math.floor(x),iy=Math.floor(y),a=smooth(x-ix),b=smooth(y-iy);
 const hash=(x,y)=>{let h=Math.imul(x,374761393)^Math.imul(y,668265263)^seed;h=Math.imul(h^(h>>>13),1274126177);return ((h^(h>>>16))>>>0)/4294967295;};
 const p=hash(ix,iy),q=hash(ix+1,iy),r=hash(ix,iy+1),s=hash(ix+1,iy+1);
 return (p+(q-p)*a)*(1-b)+(r+(s-r)*a)*b;
}
export function materialSeed(seed=''){let h=2166136261;for(const ch of String(seed))h=Math.imul(h^ch.charCodeAt(0),16777619);return h>>>0;}
export function biomeColor(p,colors=naturalMaterials,position){
 const color=name=>colors[name]||naturalMaterials[name];let name='grass';
 const x=position?.x??0,y=position?.y??0,seed=position?.seed??0;
 const broad=materialNoise(x*.31,y*.31,seed),fine=materialNoise(x*1.7+broad,y*1.7-broad,seed^9137);
 const lobe=(broad-.5)*.8+(fine-.5)*.2;
 const wet=clamp(p.moisture??.5),vegetation=clamp(p.vegetation??wet),rock=clamp(p.rock??0),sand=clamp(p.sand??0),temp=p.temperature??15;
 // One material per location. Geographic lobes shape the boundary, while
 // categorical selection prevents interpolated colors between adjacent biomes.
 if(vegetation-.50+lobe*.22>.03&&temp> -7)name='forest';
 if(.34-wet+lobe*.12>.0425&&rock<.6)name='desert';
 if(rock-.27+lobe*.17>.11)name='mountain';
 if(2-temp+lobe*5>3.5&&rock<.7)name='tundra';
 if((p.wetland??0)-.48+lobe*.08>.11)name='swamp';
 if((p.snow??0)-.26+lobe*.15>.15&&temp<.5)name='snow';
 if(sand-.2+lobe*.12>.14&&rock<.6)name='sand';
 const rgb=[...color(name)];
 return rgb;
}

export function coarseMaterial(environment,i){
 const f=environment.fields,v=(name,fallback=0)=>f[name]?.[i]??fallback;
 const rock=clamp(v('slope')*1.2+v('mountainCore')*.4);
 const sand=clamp(v('dune')+v('sandSupply')*.15*v('aridity'))*(1-rock);
 return {temperature:v('temperature',15),moisture:v('moisture',.5),vegetation:v('moisture',.5)*(1-v('snow')),rock,sand:Math.max(sand,v('beach')),snow:v('snow'),wetland:v('wetland')};
}
