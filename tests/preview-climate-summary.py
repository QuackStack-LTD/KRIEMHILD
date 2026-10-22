"""Produce comparable tables from recorded runs; never infer a cold cache."""
import json, statistics, re
from pathlib import Path

root = Path(__file__).resolve().parents[1]
out = root / 'docs/preview-climate-investigation'
load = lambda p: json.loads(p.read_text(encoding='utf8'))
seeds = ['Kriemhild','gallery-001','gallery-002','8fef5907901a','gallery-003','gallery-004','d1feae98-6510-4a72-951b-44e93a01ae86']
summary = {'preview': [], 'coldPreview': [], 'climate': [],
    'previewEvidence': 'preview-final',
    'coldPreviewEvidence': ['preview-cold (reference only)', 'preview-cold-final (balanced)']}
for path in sorted((out/'preview-final').glob('dpr-*.json')):
    data = load(path)
    for dpr in [1,2,3]:
        for width in [2,.25,.03,.003,.00008]:
            for preset in ['reference','performance','balanced','high']:
                rows = [r for r in data['runs'] if r['dpr']==dpr and r['camera']['w']==width and r['preset']==preset]
                if not rows: continue
                record = {'dpr': dpr, 'cameraWidth': width, 'preset':preset,'pixels':rows[0]['request']['pixels']}
                for cache in ['cold-browser','warm-browser']:
                    cases=[r for r in rows if r['cache']==cache]
                    record[cache]={'repetitions':len(cases),'medianMs':statistics.median(r['elapsed'] for r in cases),
                        'firstCompleteMs':statistics.median(r['frames'][0]['duration'] for r in cases),
                        'decodedFrameBytes':statistics.median(r['frames'][-1]['decodedFrameBytes'] for r in cases),
                        'workingCanvasBytes':statistics.median(r['frames'][-1]['workingCanvasBytes'] for r in cases),
                        'backendHits':sum(e['cache']=='hit' for r in cases for e in r['responses']),
                        'backendMisses':sum(e['cache']=='miss' for r in cases for e in r['responses'])}
                summary['preview'].append(record)
cold = out/'preview-cold/results.json'
if cold.exists():
    groups={}
    runs = [r for r in load(cold) if r['preset']=='reference']
    runs += load(out/'preview-cold-final/results.json')
    for run in runs:
        for r in run['runs']:
            key=(r['dpr'],r['camera']['w'],r['preset'],r['cache'])
            groups.setdefault(key,[]).append(r)
    for (dpr,width,preset,cache),rows in groups.items():
        summary['coldPreview'].append({'dpr':dpr,'cameraWidth':width,'preset':preset,'browserCache':cache,
            'repetitions':len(rows),'medianMs':statistics.median(r['elapsed'] for r in rows),
            'backendHits':sum(e['cache']=='hit' for r in rows for e in r['responses']),
            'backendMisses':sum(e['cache']=='miss' for r in rows for e in r['responses'])})
for seed in seeds:
    row={'seed':seed,'versions':{}}
    for version,folder in [(1,root/'docs/deepzoom-investigation/climate-v1'),(2,out/'paired-v2'),(3,out/'paired-v3')]:
        file=folder/(seed+'.json')
        if file.exists():
            d=load(file)
            row['versions'][str(version)]={k:d[k] for k in ['spec','biomeLandShares','latitudeBiomes','dryLandFraction','lakeFraction','fields','connectivity','generationMs']}
    if row['versions']:summary['climate'].append(row)
(out/'summary.json').write_text(json.dumps(summary,indent=2),encoding='utf8')
print('Summary:',len(summary['preview']),'preview comparisons;',len(summary['coldPreview']),'cold comparisons;',len(summary['climate']),'seeds')
