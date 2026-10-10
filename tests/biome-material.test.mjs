import {test} from 'node:test';
import assert from 'node:assert/strict';
import {biomeColor,coarseMaterial,naturalMaterials} from '../src/terrain/biome-material.js';
import {DetailTiles} from '../src/terrain/detail-tiles.js';

test('Biome boundaries select a single material without mixed colors',()=>{
 const colors=Object.values(naturalMaterials).map(c=>JSON.stringify(c));
 for(let y=0;y<20;y++)for(let x=0;x<100;x++){
  const c=biomeColor({temperature:20,moisture:.7,vegetation:x/100,rock:y/30,wetland:y/20,sand:x/150},undefined,{x:x/10,y,seed:1729});
  assert.ok(colors.includes(JSON.stringify(c)),'biome color was blended');
 }
});

test('Provisional world tiles retain physical materials while refined tiles load',()=>{
 const fields=Object.fromEntries(['elevation','waterBody','waterDepth','waterLevel','temperature','moisture','slope','mountainCore','snow','wetland','aridity','sandSupply'].map(k=>[k,Array(4).fill(0)]));
 fields.moisture.fill(.8);fields.slope.fill(.5);fields.mountainCore.fill(.9);fields.snow.fill(.4);
 const solver={W:2,H:2,environment:{fields}},tiles=new DetailTiles(solver,()=>{});
 const p=tiles.cache.get('0/0/0').points[0];
 assert.equal(p.moisture,.8);assert.equal(p.snow,.4);assert.ok(p.rock>.9);assert.ok(p.vegetation>0);
 tiles.dispose();
});
test('Sand supply does not paint entire mountain cells or lake borders as beach',()=>{
 const env={fields:{slope:[.6,.01],mountainCore:[.8,0],dune:[0,0],sandSupply:[1,1],aridity:[.5,.5],beach:[0,0],moisture:[.5,.5]}};
 const mountain=coarseMaterial(env,0),plain=coarseMaterial(env,1);
 assert.ok(mountain.sand<.02);assert.ok(plain.sand<.1);assert.ok(mountain.rock>.9);
});

test('Ecotones have geographic lobes with a narrow readable boundary',()=>{
 const crossings=[],bands=[];for(let y=0;y<40;y+=.5){let crossing=null,band=0;for(let x=0;x<=10;x+=.05){const p={moisture:.7,vegetation:.30+x*.05,rock:0,temperature:20},c=biomeColor(p,undefined,{x,y,seed:1729});const forest=(113-c[0])/60;if(forest>.1&&forest<.9)band++;if(crossing===null&&forest>=.5)crossing=x;}crossings.push(crossing);bands.push(band*.05);}
 assert.ok(bands.reduce((a,b)=>a+b)/bands.length<1.2,'ecotone is a broad blur');
 assert.ok(Math.max(...crossings)-Math.min(...crossings)>.8,'boundary remained straight');
 const p={moisture:.7,vegetation:.56,temperature:20};assert.deepEqual(biomeColor(p,undefined,{x:12.25,y:8.5,seed:1729}),biomeColor(p,undefined,{x:12.25,y:8.5,seed:1729}));
});
test('Waterlogged swamps differ clearly from forests, ordinary floodplains do not become swamps',()=>{
 const p={moisture:.85,vegetation:.9,temperature:22,rock:0};
 const forest=biomeColor(p),swamp=biomeColor({...p,wetland:.9}),plain=biomeColor({...p,floodplain:1});
 assert.ok(Math.hypot(...swamp.map((v,i)=>v-forest[i]))>35);
 assert.deepEqual(plain,forest);
});
