"""Record source/build identity and the final, controlled evidence datasets."""
from pathlib import Path
import datetime
import hashlib
import json
import platform
import subprocess

root = Path(__file__).resolve().parents[1]
out = root / 'docs/preview-climate-investigation'

def command(*args):
    return subprocess.check_output(args, cwd=root, text=True).strip()

def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1048576), b''):
            digest.update(block)
    return digest.hexdigest()

paths = command('git', 'ls-files', '--modified', '--others', '--exclude-standard').splitlines()
source = {p: sha(root / p) for p in paths
          if Path(p).suffix in {'.go', '.js', '.jsx', '.mjs', '.css', '.py', '.ps1'}
          and (root / p).is_file()}
evidence = [out / 'REPORT.md', out / 'EXPERIMENTS.md', out / 'summary.json']
for folder in ['preview-final', 'preview-cold', 'preview-cold-final',
               'preview-cold-natural', 'preview-memory-final', 'preview-integration',
               'climate-ui', 'climate-ui-repeat', 'paired-v2', 'paired-v3',
               'fixed-surface-v2', 'fixed-surface-v3']:
    evidence.extend((out / folder).glob('*.json'))
build = [root / '.cache/kriemhild-preview-climate.exe', root / 'dist/index.html']
build.extend((root / 'dist/assets').glob('*'))
result = {
    'utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'branch': command('git', 'branch', '--show-current'),
    'commit': command('git', 'rev-parse', 'HEAD'),
    'dirtyStatus': command('git', 'status', '--short'),
    'os': platform.platform(), 'node': command('node', '--version'),
    'go': command(str(root / '.tools/go/bin/go.exe'), 'version'),
    'sourceSHA256': source,
    'buildSHA256': {p.relative_to(root).as_posix(): sha(p) for p in build if p.is_file()},
    'evidenceSHA256': {p.relative_to(out).as_posix(): sha(p) for p in evidence},
    'renderConfiguration': {
        'workers': 3, 'backendSlots': 3, 'physicalNodes': 8,
        'encodedCacheBytes': 67108864, 'outputPixelBudget': 33554432,
        'atlasPixelBudget': 67108864, 'maxLOD': 18,
        'previewDensity': 'CSS pixels, independent of screen DPR',
        'defaultPreviewScale': .75, 'customScaleRange': [.5, 1.5],
        'illustratorMaxDPR': 3,
    },
    'climate': {'default': 1, 'candidate': 3, 'candidateStatus': 'experimental'},
    'servers': {
        '8124': 'original backend and original data left untouched',
        '8125': 'previous investigation review backend retained',
        '8127': '.cache/quality-climate-20261022 disposable review store',
        '8128': 'owned diagnostic backend stopped after measurements',
    },
}
(out / 'final-manifest.json').write_text(json.dumps(result, indent=2), encoding='utf8')
print('Recorded', len(source), 'source hashes and', len(evidence), 'evidence hashes')
