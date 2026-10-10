import {test} from 'node:test';
import assert from 'node:assert/strict';
import {biomeColor,coarseMaterial} from '../src/terrain/biome-material.js';
import {DetailTiles} from '../src/terrain/detail-tiles.js';

test('Biome material transitions have no moisture, mountain or temperature cutoff',()=>{
 const p={temperature:15,moisture:.5,vegetation:.5,rock:.3,sand:0,snow:0};
 for(const field of ['moisture','vegetation','rock','sand','snow','temperature'])for(const value of [.28,.3,.43,.65,0,1,5]){
  const a=biomeColor({...p,[field]:value-1e-6}),b=biomeColor({...p,[field]:value+1e-6});
  assert.ok(Math.max(...a.map((v,k)=>Math.abs(v-b[k])))<.002,`${field} ${value}`);
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
