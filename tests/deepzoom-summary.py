import json, pathlib, statistics, hashlib
root=pathlib.Path('docs/deepzoom-investigation')
read=lambda p: json.loads(p.read_text(encoding='utf8'))
seeds=['Kriemhild','gallery-001','gallery-002','8fef5907901a','gallery-003','gallery-004','d1feae98-6510-4a72-951b-44e93a01ae86']
rows=[]
for seed in seeds:
    a,b=[read(root/d/(seed+'.json')) for d in ['climate-v1','climate-v2']]
    keys=['forest','rainforest','temperate-rainforest','steppe','desert','tundra','woodland','shrubland']
    row={'seed':seed,'biomes':{k:[a['biomeLandShares'].get(k,0),b['biomeLandShares'].get(k,0)] for k in keys},
         'dryLand':[a['dryLandFraction'],b['dryLandFraction']], 'lake':[a['lakeFraction'],b['lakeFraction']],
         'meanTemperature':[a['fields']['temperature']['Mean'],b['fields']['temperature']['Mean']],
         'meanPET':[a['fields']['pet']['Mean'],b['fields']['pet']['Mean']]}
    previous=pathlib.Path('docs/viewport-investigation/climate-v1-final')/(seed+'.json')
    if previous.exists():
        p=read(previous)
        row['v1Compatibility']={k:p[k]==a[k] for k in ['biomeLandShares','fields','latitudeBiomes','connectivity','landFraction','lakeFraction']}
        row['v1ImageHashesMatch']={style:hashlib.sha256((previous.parent/(seed+'-'+style+'.png')).read_bytes()).digest()==hashlib.sha256((root/'climate-v1'/(seed+'-'+style+'.png')).read_bytes()).digest() for style in ['biome','temperature','precipitation','aridity','seasonality','elevation']}
    rows.append(row)
summary={'climate':rows}
for name in ['browser-measured','browser-after-verified']:
    path=root/name/'results.json'
    if not path.exists():continue
    runs=read(path);groups={}
    for run in runs:
        for s in run['stages']:
            key=run['cache']+'/'+s['name'];g=groups.setdefault(key,{'gestureMS':[],'finalMS':[],'coarseMS':[],'timings':[]})
            g['gestureMS'].append(s['duration'])
            for f in s['log']:
                if f['event']=='frame':
                    g[f['phase']+'MS'].append(f['duration'])
                    if f['phase']=='final':g['timings'].append(f.get('timings'))
    summary[name]={k:{field:{'n':len(v),'median':statistics.median(v),'min':min(v),'max':max(v)} if field!='timings' and v else v for field,v in g.items()} for k,g in groups.items()}
(root/'summary.json').write_text(json.dumps(summary,indent=2),encoding='utf8')
for row in rows:print(row['seed'],{k:[round(100*x,2) for x in v] for k,v in row['biomes'].items()})
