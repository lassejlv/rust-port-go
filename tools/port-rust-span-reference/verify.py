"""Supporting archive/expanded-source verification; no source-file credit."""
import hashlib,io,pathlib,tarfile,tomllib
def sha(data):return hashlib.sha256(data).hexdigest()
def verify_registry(metadata,lock,cargo_home):
 locked={(p['name'],p['version']):p for p in tomllib.loads(lock.decode())['package'] if p.get('source','').startswith('registry+')}
 retained=[]
 for package in metadata['packages']:
  if not package.get('source','') or not package['source'].startswith('registry+'):continue
  key=(package['name'],package['version']);entry=locked[key]
  source=pathlib.Path(package['manifest_path']).parent
  if source.is_symlink() or not source.is_dir():raise RuntimeError(f'nonregular crate root: {source}')
  # Cargo locates registry/src/<registry>/<name-version>; the matching archive
  # must be under that same registry, never an arbitrary archive name search.
  archive=cargo_home/'registry/cache'/source.parent.name/(source.name+'.crate')
  blob=archive.read_bytes()
  if sha(blob)!=entry['checksum']:raise RuntimeError(f'archive checksum changed: {key}')
  expected={}
  with tarfile.open(fileobj=io.BytesIO(blob),mode='r:gz') as tar:
   prefix=source.name+'/'
   for member in tar:
    if member.name==source.name or member.name==prefix:continue
    if not member.name.startswith(prefix):raise RuntimeError(f'archive path outside crate: {member.name}')
    name=member.name[len(prefix):]
    path=pathlib.PurePosixPath(name)
    if '..' in path.parts or path.is_absolute():raise RuntimeError(f'unsafe crate path: {name}')
    if member.isdir():continue
    if not member.isfile():raise RuntimeError(f'nonregular crate member: {name}')
    if name in expected:raise RuntimeError(f'duplicate crate member: {name}')
    expected[name]=tar.extractfile(member).read()
  actual=set()
  for path in source.rglob('*'):
   name=path.relative_to(source).as_posix()
   if path.is_symlink():raise RuntimeError(f'crate source symlink: {path}')
   if path.is_dir():continue
   if not path.is_file():raise RuntimeError(f'nonregular crate source: {path}')
   if name=='.cargo-ok':continue
   actual.add(name)
   if name not in expected or path.read_bytes()!=expected[name]:raise RuntimeError(f'compiled registry source differs: {path}')
  if actual!=expected.keys():raise RuntimeError(f'missing registry sources for {key}: {expected.keys()-actual}')
  retained.append({'name':key[0],'version':key[1],'checksum':entry['checksum'],'compiled_files':len(actual)})
 return sorted(retained,key=lambda p:p['name'])
