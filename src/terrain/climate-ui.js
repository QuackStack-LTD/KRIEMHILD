export const climateInputs={planetTilt:'axialTilt',planetYearDays:'orbitalDays',planetEccentricity:'eccentricity',planetPerihelion:'perihelion',planetCirculation:'circulation',planetRadius:'radiusKm',longitudeWest:'longitudeWest',longitudeEast:'longitudeEast'};
export const climateSettingIds=['seasonMode','seasonCount','seasonNames','climateCoverage','worldScale','riverMinArea','riverMinDischarge','riverVisibleOrder','riverMajorArea','riverRegionalArea','ephemeralDensity','erosionIterations','erosionDuration','erosionStrength',...Object.keys(climateInputs)];
export function climateSettings(values,width,height,pixelsPerCell){
 const settings={mode:values.seasonMode||'automatic',width_px:width*pixelsPerCell,height_px:height*pixelsPerCell,pixels_per_cell:pixelsPerCell};
 if(settings.mode==='custom'){settings.count=Number(values.seasonCount);if(values.seasonNames?.trim())settings.names=values.seasonNames.split(',').map(v=>v.trim());}
 if(values.climateCoverage)settings.coverage=values.climateCoverage;
 for(const [id,key] of Object.entries(climateInputs)){if(values[id]!==''&&values[id]!==undefined&&values[id]!==null)settings[key]=Number(values[id]);}
 return settings;
}
export function renderClimateDescription(data,element){
 const section=element('details','');section.className='climate-description';section.open=true;
 section.append(element('summary','Climate & seasons'));
 if(!data?.available){section.append(element('p',data?.summary||'No stored seasonal climate layer in this terrain.'));return section;}
 section.append(element('strong',data.zone),element('p',data.summary));
 section.append(element('small',`${data.approximate?'Approximate procedural climate':'Procedural climate'}; modeled averages, not individual weather events. Tilt ${data.axialTilt.toFixed(1)}°, year ${data.yearDays.toFixed(0)} days. Grid spacing approximately ${data.resolutionKm.map(v=>v.toFixed(1)).join(' × ')} km here.`));
 if(data.transitionZones?.length)section.append(element('p',`Transition toward ${data.transitionZones.join(', ')}.`));
 for(const text of new Set(data.influences||[]))section.append(element('p',text));
 for(const season of data.seasons||[]){const entry=element('details','');entry.append(element('summary',`${season.name} · ${season.durationDays.toFixed(0)} days · starts month ${season.startMonth+1}`),element('p',season.description));section.append(entry);}
 if(data.localCount!==data.calendarCount)section.append(element('small',`The reference land calendar has ${data.calendarCount} phases; this location has ${data.localCount}. Southern and northern seasonal timing may differ.`));
 return section;
}

export function geographicSettings(values,width,height,pixelsPerCell){
 const g={width_px:width*pixelsPerCell,height_px:height*pixelsPerCell,pixels_per_cell:pixelsPerCell,map_coverage:values.climateCoverage||'planet',projection:'equirectangular'};
 for(const [id,key] of Object.entries({planetRadius:'planetary_radius',longitudeWest:'longitude_west',longitudeEast:'longitude_east',worldScale:'world_scale'}))if(values[id]!==''&&values[id]!=null)g[key]=Number(values[id]);
 return g;
}
export function riverSettings(values){
 const r={};for(const [id,key] of Object.entries({riverMinArea:'minimum_area_km2',riverMinDischarge:'minimum_discharge_m3s',riverVisibleOrder:'visible_stream_order',riverMajorArea:'major_area_km2',riverRegionalArea:'regional_area_km2',ephemeralDensity:'ephemeral_density'}))if(values[id]!==''&&values[id]!=null)r[key]=Number(values[id]);return r;
}
export function erosionSettings(values){return {iterations:Number(values.erosionIterations??2),duration_ma:Number(values.erosionDuration??2),strength:Number(values.erosionStrength??1)};}
