const number=v=>Number(v||0).toLocaleString(undefined,{maximumFractionDigits:1});
const distribution=d=>d?`${number(d.min)} / ${number(d.median)} / ${number(d.p90)} / ${number(d.max)}`:'Unavailable';

export function hydrologyReportRows(environment){
 const h=environment?.hydrology,s=h?.statistics;if(!s)return [];
 const rows=s.classes.map(c=>[c.class==='major'?'Major rivers':c.class==='regional'?'Regional rivers':'Local rivers / streams',`${c.count} · total length ${number(c.length)}`]);
 rows.push(['Connected drainage basins',number(s.basinCount)],['Lakes / wetlands / springs',`${s.lakes} / ${s.wetlands} / ${s.springs}`],['Drainage coverage',`${number(s.coveragePercent)}% of land · target ${number(s.targetCoveragePercent)}%`],['Invalid river destinations',number(s.invalidDestinations)],['Headwaters outside mountain cores',number(s.lowlandHeadwaters)],['River lengths: min / median / P90 / max',distribution(s.lengths)],['Contributing areas: min / median / P90 / max',distribution(s.catchments)],['Basin sizes: min / median / P90 / max',distribution(s.basinSizes)]);
 if(environment.geography){const g=environment.geography;rows.push(['Geographic coverage',`${g.map_coverage} ? radius ${number(g.planetary_radius)} km ? ${g.projection}`],['Grid spacing',`${number(g.world_scale)} km north?south per cell; east?west spacing and area vary with latitude`]);}if(environment.geomorphology){const g=environment.geomorphology;rows.push(['River erosion',`${g.iterations} passes over ${number(g.parameters.duration_ma)} million years ? ${g.landforms.length} recorded landforms`],['Sediment budget',`${number(g.erodedM3/1e9)} km? eroded / ${number(g.depositedM3/1e9)} km? deposited / ${number(g.exportedM3/1e9)} km? exported or stored beyond the resolved bed`]);}
 const targets=h.targets||[],sum=key=>targets.reduce((total,t)=>total+(t[key]||0),0);
 rows.push(['Lake area / habitat target',`${sum('lakeCells')} / ${sum('targetLakeCells')}`],['Wetland area / habitat target',`${sum('wetlandCells')} / ${sum('targetWetlandCells')}`],['Units',`${s.lengthUnits} for length; ${s.areaUnits} for area.`]);
 for(const note of s.notes||[])rows.push(['Coverage note',note]);
 rows.push(['Water-body targets','Habitat targets are conditional: unsuitable terrain or insufficient water is never flooded to meet a quota.']);
 return rows;
}

export function updateHydrologyReport(container,environment){
 if(!container||container.hydrology===environment?.hydrology)return;
 container.hydrology=environment?.hydrology;container.replaceChildren();
 const rows=hydrologyReportRows(environment);container.hidden=!rows.length;if(!rows.length)return;
 const summary=document.createElement('summary');summary.textContent='Hydrology report';container.append(summary);
 const dl=document.createElement('dl');for(const [label,value] of rows){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=value;dl.append(dt,dd);}container.append(dl);
}
