import {test} from 'node:test';
import assert from 'node:assert/strict';
import {waterSurfacePositions} from '../src/terrain/water-surface.js';
import {reliefShades} from '../src/terrain/relief.js';

test('Water mesh excludes dry depressions and respects local lake levels',()=>{
  const fields={elevation:[-100,-300,-500,40],waterBody:[0,1,2,0],waterLevel:[0,0,200,0],waterDepth:[0,300,700,0]};
  const mesh=waterSurfacePositions(fields,2,2,2,false);
  assert.equal(mesh.length,36); // exactly two wet cells, not a global sea plane
  for(let i=1;i<18;i+=3)assert.ok(Math.abs(mesh[i]-.002)<1e-6);
  for(let i=19;i<36;i+=3)assert.ok(Math.abs(mesh[i]-.202)<1e-6);
  assert.ok([...mesh].every(Number.isFinite));
  const sphere=waterSurfacePositions(fields,2,2,2,true);
  assert.equal(sphere.length,36);assert.ok([...sphere].every(Number.isFinite));
});

test('Depth shading distinguishes shallow and deep water; lowlands retain relief',()=>{
  const e={fields:{elevation:[-10,-100,-1000,10,140,30,100,350,90],waterDepth:[10,100,1000,0,0,0,0,0,0]}};
  const shades=reliefShades(e,3,3);
  assert.ok(shades[0]>shades[1]&&shades[1]>shades[2]);
  assert.notEqual(shades[3],shades[4]);
});
