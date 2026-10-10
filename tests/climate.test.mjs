import {test} from 'node:test';
import assert from 'node:assert/strict';
import {climateSettings,renderClimateDescription,geographicSettings,riverSettings,erosionSettings} from '../src/terrain/climate-ui.js';

test('Climate settings distinguish generated defaults from explicit zero forcing',()=>{
 const a=climateSettings({seasonMode:'automatic',planetTilt:'',planetEccentricity:'0'},160,100,6);
 assert.equal(a.width_px/a.pixels_per_cell,160);assert.equal(a.height_px/a.pixels_per_cell,100);
 assert.equal(a.axialTilt,undefined);assert.equal(a.eccentricity,0);assert.equal(a.count,undefined);
 const b=climateSettings({seasonMode:'custom',seasonCount:'5',seasonNames:'Ember, Rain, Frost, Thaw, Mist',planetTilt:'0',planetYearDays:'700',climateCoverage:'island'},160,100,12);
 assert.equal(b.count,5);assert.equal(b.axialTilt,0);assert.equal(b.orbitalDays,700);assert.equal(b.coverage,'island');
 assert.deepEqual(b.names,['Ember','Rain','Frost','Thaw','Mist']);
});

test('Climate panel uses modeled summaries and stored seasonal descriptions',()=>{
 const element=(tag,text)=>({tag,text,children:[],append(...children){this.children.push(...children)}});
 const panel=renderClimateDescription({available:true,approximate:true,zone:'Monsoon',summary:'Warm wet/dry climate.',axialTilt:15,yearDays:400,resolutionKm:[10,12],transitionZones:['Rainforest'],influences:['Coastal moderation.','Coastal moderation.'],localCount:2,calendarCount:4,seasons:[{name:'Wet season',durationDays:250,startMonth:2,description:'Modeled rainfall 1100 mm.'}]},element);
 const flatten=node=>[node.text,...node.children.flatMap(flatten)];const texts=flatten(panel);
 assert.ok(texts.includes('Modeled rainfall 1100 mm.'));assert.equal(texts.filter(v=>v==='Coastal moderation.').length,1);
 assert.match(texts.join(' '),/modeled averages, not individual weather events/);
 assert.match(flatten(renderClimateDescription(null,element)).join(' '),/No stored seasonal climate layer/);
});

test('Planetary configuration keeps geographic and river units explicit',()=>{
 assert.deepEqual(geographicSettings({},160,100,6),{width_px:960,height_px:600,pixels_per_cell:6,map_coverage:'planet',projection:'equirectangular'});
 assert.equal(geographicSettings({worldScale:'2',climateCoverage:'local'},160,100,6).world_scale,2);
 assert.deepEqual(riverSettings({riverMinArea:'500',ephemeralDensity:'0'}),{minimum_area_km2:500,ephemeral_density:0});
 assert.deepEqual(erosionSettings({erosionIterations:'0',erosionDuration:'4',erosionStrength:'1'}),{iterations:0,duration_ma:4,strength:1});
});
