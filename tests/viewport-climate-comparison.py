"""Compare fixed-seed dry-land statistics and authoritative diagnostic rasters."""
from pathlib import Path
import json, hashlib
out = Path('docs/viewport-investigation')
seeds = ['Kriemhild', 'gallery-001', 'gallery-002', '8fef5907901a', 'gallery-003', 'gallery-004']
result = []
for seed in seeds:
    previous = Path('docs/terrain-investigation/climate-final') if seed in seeds[:3] else out/'climate-v1-baseline'
    version0 = Path('docs/terrain-investigation/climate-before') if seed in seeds[:3] else out/'climate-v0'
    after = out/'climate-v1-final'
    before, final, old = [json.loads((p/(seed+'.json')).read_text()) for p in [previous, after, version0]]
    keys = ['spec', 'landFraction', 'dryLandFraction', 'lakeFraction', 'oceanFraction', 'biomeLandShares', 'latitudeBiomes', 'fields', 'connectivity']
    equality = {key: before[key] == final[key] for key in keys}
    hashes = {}
    for style in ['biome','temperature','precipitation','aridity','seasonality','elevation']:
        a,b=[hashlib.sha256((p/f'{seed}-{style}.png').read_bytes()).hexdigest() for p in [previous,after]]
        hashes[style]={'before':a,'after':b,'equal':a==b}
    assert all(equality.values()), (seed,equality)
    assert all(h['equal'] for h in hashes.values()), (seed,hashes)
    # Final cold-path optimization is also checked on the user's saved seed.
    optimized = out/'climate-cold-fast-path'
    if (optimized/(seed+'.json')).exists():
        last=json.loads((optimized/(seed+'.json')).read_text())
        assert all(final[k]==last[k] for k in keys), seed
        assert all(hashlib.sha256((optimized/f'{seed}-{style}.png').read_bytes()).hexdigest()==h['after'] for style,h in hashes.items()), seed
    result.append({'seed':seed,'beforeDirectory':str(previous),'statisticsEqual':equality,'hashes':hashes,'v0':old['biomeLandShares'],'v1':final['biomeLandShares'],'dryLandV0':old['dryLandFraction'],'dryLandV1':final['dryLandFraction'],'lakeV0':old['lakeFraction'],'lakeV1':final['lakeFraction']})
(out/'climate-comparison.json').write_text(json.dumps(result,indent=2))
print('All 6 seeds: 36 raster hashes and all climate, latitude, biome and connectivity statistics unchanged.')
for r in result:
    print(r['seed'], ' / '.join(f'{k}: {100*r["v0"].get(k,0):.1f} -> {100*r["v1"].get(k,0):.1f}' for k in ['forest','woodland','steppe','desert','temperate-rainforest','rainforest','shrubland']))
