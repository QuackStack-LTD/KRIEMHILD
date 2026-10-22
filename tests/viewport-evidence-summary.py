from pathlib import Path
import json,statistics
root=Path('docs/viewport-investigation')
runs=json.loads((root/'benchmark/results.json').read_text())
rows=[]
for old in [False,True]:
 for renderer in ['tiles','viewport']:
  group=[r for r in runs if r['old']==old and r['renderer']==renderer]
  assert len(group)==3 and all(not r['errors'] for r in group)
  row={'source':'continuous-1' if old else 'continuous-3','renderer':renderer,'latency':{},'frames':[]}
  for stage in ['global','regional','close','out','return']:
   vals=[next(s for s in r['stages'] if s['name']==stage)['ms'] for r in group]
   row['latency'][stage]={'min':min(vals),'median':statistics.median(vals),'max':max(vals)}
  for r in group:
   for s in r['stages']:
    if s.get('frame'): row['frames'].append({k:s['frame'][k] for k in ['duration','bytes','cacheBytes','hits','misses','phase','pixels']})
  rows.append(row)
 # Exact camera/viewport matching between both implementations, every repetition.
 expected=[(s['camera'],s['width'],s['height']) for s in next(r for r in runs if r['old']==old)['stages']]
 for r in (r for r in runs if r['old']==old):
  assert [(s['camera'],s['width'],s['height']) for s in r['stages']]==expected
(root/'performance-summary.json').write_text(json.dumps(rows,indent=2))
for r in rows: print(r['source'],r['renderer'],{s:round(v['median']) for s,v in r['latency'].items()})
