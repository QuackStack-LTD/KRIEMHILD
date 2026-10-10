const clamp=v=>Math.max(0,Math.min(1,v));
const smooth=v=>{v=clamp(v);return v*v*(3-2*v);};
export const naturalMaterials=Object.freeze({grass:[113,148,78],forest:[53,103,59],desert:[158,141,99],mountain:[123,119,109],tundra:[143,150,127],snow:[224,232,231],sand:[191,172,124],swamp:[77,107,72]});

// Continuous physical materials replace hard, single-biome polygon fills.
// Neither zoom level nor a mountain/shore classification selects a color band.
export function biomeColor(p,colors=naturalMaterials){
 const c={...naturalMaterials,...colors};let rgb=[...c.grass];
 const mix=(name,weight)=>{weight=clamp(weight);rgb=rgb.map((v,k)=>v+(c[name][k]-v)*weight);};
 const wet=clamp(p.moisture??.5),vegetation=clamp(p.vegetation??wet),rock=clamp(p.rock??0),sand=clamp(p.sand??0),temp=p.temperature??15;
 mix('forest',smooth((vegetation-.3)/.6)*(1-smooth((-temp-3)/15)));
 mix('desert',smooth((.43-wet)/.38)*(1-rock*.65));
 mix('mountain',rock);
 mix('tundra',smooth((5-temp)/16)*(1-rock*.6));
 mix('swamp',clamp(p.floodplain??0)*.3+clamp(p.wetland??0)*.6);
 mix('snow',smooth((p.snow??0)*1.3)*smooth((5-temp)/12));
 mix('sand',sand*(1-rock*.65));
 return rgb;
}

export function coarseMaterial(environment,i){
 const f=environment.fields,v=(name,fallback=0)=>f[name]?.[i]??fallback;
 const rock=clamp(v('slope')*1.2+v('mountainCore')*.4);
 const sand=clamp(v('dune')+v('sandSupply')*.15*v('aridity'))*(1-rock);
 return {temperature:v('temperature',15),moisture:v('moisture',.5),vegetation:v('moisture',.5)*(1-v('snow')),rock,sand:Math.max(sand,v('beach')),snow:v('snow'),wetland:v('wetland')};
}
