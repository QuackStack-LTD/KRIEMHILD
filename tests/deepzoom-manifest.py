"""Record exact uncommitted source/build identity for the completed experiments."""
from pathlib import Path
import hashlib,json,platform,subprocess,datetime

root=Path(__file__).resolve().parents[1]
def command(*args):return subprocess.check_output(args,cwd=root,text=True).strip()
def sha(path):
    digest=hashlib.sha256()
    with path.open('rb') as f:
        for block in iter(lambda:f.read(1024*1024),b''):digest.update(block)
    return digest.hexdigest()
files=command('git','ls-files','--modified','--others','--exclude-standard').splitlines()
code={p:sha(root/p) for p in files if Path(p).suffix in {'.go','.js','.jsx','.mjs','.css','.py'} and (root/p).is_file()}
result={'utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'branch':command('git','branch','--show-current'),
        'commit':command('git','rev-parse','HEAD'),'dirtyStatus':command('git','status','--short'),
        'os':platform.platform(),'node':command('node','--version'),'go':command(str(root/'.tools/go/bin/go.exe'),'version'),
        'sourceSHA256':code,'backendSHA256':sha(root/'.cache/kriemhild-deepzoom-review.exe'),
        'frontendIndexSHA256':sha(root/'dist/index.html'),
        'authoritativeMetadataSHA256':sha(root/'.cache/deepzoom-planet/metadata'),
        'authoritativeFieldsSHA256':sha(root/'.cache/deepzoom-planet/fields'),
        'renderConfiguration':{'workers':3,'backendSlots':3,'physicalNodes':8,'encodedCacheBytes':67108864,'outputPixelBudget':33554432,'atlasPixelBudget':67108864,'maxDPR':3,'maxLOD':18},
        'testServer':'8125: .cache/deepzoom-20261022T122540Z','comparisonServer':'8126: .cache/deepzoom-comparison (stopped)',
        'userServer':'8124: left running; original data not authored by tests'}
(root/'docs/deepzoom-investigation/final-manifest.json').write_text(json.dumps(result,indent=2),encoding='utf8')
print('Recorded final source and build hashes')
