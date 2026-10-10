import {test} from 'node:test';
import assert from 'node:assert/strict';
import {hydrologyReportRows} from '../src/terrain/hydrology-report.js';
import {fieldColor,overlayLegend} from '../src/terrain/environment-view.js';
import {drawDetailFeatures} from '../src/terrain/detail-render.js';
import {builderPreferences} from '../src/builder/preferences.js';

test('Small worlds with no springs or canals still render their terrain legend',()=>{
 const legend=overlayLegend({hydrology:{reaches:null,springs:null,wetlands:null,canals:null,diagnostics:null}},'terrain');
 assert.match(legend,/0 connected river reaches/);
 assert.match(legend,/Hydrology validation passed/);
});

test('Hydrology reports count whole rivers and declare geographic units',()=>{
 const distribution={min:1,median:3,p90:20,max:80};
 const environment={hydrology:{targets:[],statistics:{classes:[{class:'major',count:2,length:140},{class:'regional',count:10,length:72},{class:'stream',count:30,length:60}],basinCount:55,lakes:8,wetlands:12,springs:5,coveragePercent:8,targetCoveragePercent:10,invalidDestinations:0,lowlandHeadwaters:25,lengths:distribution,catchments:distribution,basinSizes:distribution,lengthUnits:'base-grid cells',areaUnits:'base-grid cells squared',notes:[]}}};
 const rows=Object.fromEntries(hydrologyReportRows(environment));
 assert.match(rows['Major rivers'],/^2 · total length 140$/);assert.match(rows['Units'],/base-grid cells/);assert.equal(rows['Invalid river destinations'],'0');
 assert.deepEqual(hydrologyReportRows({}),[]);
});

test('Watershed and river-system debug layers distinguish identities and persist',()=>{
 const e={fields:{watershed:[42,99],riverSystem:[1,2],ocean:[0,0],riverClass:[1,3]}};
 assert.notEqual(fieldColor(e,'watershed',0),fieldColor(e,'watershed',1));
 assert.notEqual(fieldColor(e,'riverClass',0),fieldColor(e,'riverClass',1));
 assert.match(overlayLegend(e,'riverSystem'),/Tributaries/);
 for(const overlay of ['watershed','accumulation','riverClass','riverSystem'])assert.equal(builderPreferences({tools:{overlay}}).overlay,overlay);
});

test('River debug view reveals connected streams at world scale with flow arrows',()=>{
 let fills=0;const ctx={save(){},restore(){},beginPath(){},moveTo(){},lineTo(){},closePath(){},fill(){fills++;}};
 const feature={id:'reach',riverId:'main',class:'major',kind:'river',level:6,width:.1,path:[[0,0,80],[1,0,70],[2,0,60]]};
 drawDetailFeatures(ctx,[{tile:{features:[feature]},arrival:1}],{x:0,y:0,scale:20},0,'riverClass');
 assert.equal(fills,2,'channel ribbon and downstream arrow should render');
});
