import test from 'node:test';
import assert from 'node:assert/strict';
import {interpretIdeas,catalogueRows,habitatStats,blankLifeform} from '../src/worldbuilding/biosphere.js';
test('guided ideas produce reviewable targeted biological rules, not invented climate',()=>{
 const result=interpretIdeas('In geothermal valleys, luminous fungi and heat-tolerant plants. Herbivores feed on fungi.');
 assert.equal(result.rules.length,3);
 const fungus=result.rules.find(r=>r.targetGroup==='fungus');
 assert.deepEqual(fungus.habitats,['geothermal']);assert.ok(fungus.traits.bioluminescence);assert.equal(fungus.tolerances,undefined);
 const animal=result.rules.find(r=>r.targetGroup==='animal');assert.equal(animal.foodGroup,'fungus');assert.deepEqual(animal.targetRoles,['herbivore']);
 assert.equal(interpretIdeas('Singing crystal membranes').rules.length,0);
});
test('catalogue uses Age-specific identity and saved occurrences rather than inferred suitability',()=>{
 const biology=blankLifeform();const doc={entities:[{id:'a',ageId:'old',name:'Former tree',biology},{id:'a',ageId:'new',name:'New tree',biology},{id:'b',ageId:'new',name:'Rare tree',biology}]};
 const state={habitats:[{id:'forest'},{id:'desert'}],occurrences:[{speciesId:'a',habitatId:'forest',status:'native'},{speciesId:'b',habitatId:'forest',status:'locally-extinct'}]};
 assert.equal(catalogueRows(doc,'new',{habitat:'desert'},state).length,0);
 assert.equal(catalogueRows(doc,'new',{search:'new tree'},state)[0].ageId,'new');
 assert.equal(catalogueRows(doc,'new',{status:'locally-extinct'},state)[0].id,'b');
 assert.deepEqual(habitatStats(state),{habitats:2,species:1,occurrences:2,incomplete:1});
});
