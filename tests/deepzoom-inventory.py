"""Read-only source inventory and a new, consistent disposable SQLite backup."""
import json, sqlite3, subprocess, datetime
from pathlib import Path

root = Path(__file__).resolve().parents[1]
out = root / 'docs/deepzoom-investigation'
out.mkdir(exist_ok=True)
run = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
destination = root / '.cache' / ('deepzoom-' + run)
destination.mkdir()
source = sqlite3.connect((root / 'data/kriemhild.sqlite').as_uri() + '?mode=ro', uri=True)
backup = sqlite3.connect(destination / 'kriemhild.sqlite')
source.backup(backup)
source.close()
result = {'run': run, 'source': 'data/kriemhild.sqlite (read-only)', 'backup': str(destination),
          'commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(),
          'branch': subprocess.check_output(['git', 'branch', '--show-current'], cwd=root, text=True).strip(),
          'status': subprocess.check_output(['git', 'status', '--short'], cwd=root, text=True), 'tables': {}}
for name in ['history_worlds', 'history_ages', 'history_terrains', 'history_maps', 'kriemhild_worlds']:
    if not backup.execute('select 1 from sqlite_master where name=?', (name,)).fetchone(): continue
    backup.row_factory = sqlite3.Row
    rows = []
    for row in backup.execute('select * from ' + name):
        item = dict(row)
        for key, value in list(item.items()):
            if isinstance(value, bytes): item[key] = {'bytes': len(value)}
            elif key == 'data' and isinstance(value, str):
                try: item[key] = json.loads(value)
                except ValueError: pass
        rows.append(item)
    result['tables'][name] = rows
result['terrains'] = []
for row in backup.execute("select world_id,data from kriemhild_world_parts where path='continuous/metadata'"):
    metadata = json.loads(row['data'])
    result['terrains'].append({'project': row['world_id'], 'metadata': metadata})
backup.close()
(out / 'inventory.json').write_text(json.dumps(result, indent=2), encoding='utf8')
print(json.dumps({'backup': str(destination), 'worlds': result['tables']['history_worlds'],
                 'links': result['tables']['history_terrains'],
                 'specs': [{'project': p['project'], 'spec': p['metadata']['spec']} for p in result['terrains']]}, indent=2))
