#!/usr/bin/env python3
import argparse,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('table_support',HERE/'generate.py');SUPPORT=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(SUPPORT)
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args();proof=None;profiles=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-hash-one-')as temporary:
  for index,(name,fallback,version)in enumerate(SUPPORT.PROFILES):
   rustc=SUPPORT.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),fallback)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Reference compiler differs')
   exe=str(pathlib.Path(temporary)/name)
   subprocess.run([rustc,'--edition=2024',str(HERE/'hash_one_oracle.rs'),'-o',exe],env=dict(os.environ,RUSTC_BOOTSTRAP='1',RUSTFLAGS=''),check=True)
   current=subprocess.check_output([exe])
   if proof is not None and current!=proof:raise SystemExit('Actual hash_one profiles differ')
   proof=current;profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'raw_sha256':sha(current)})
 manifest={'scope':'Actual std BuildHasher::hash_one override dispatch, owned and borrowed keys; scoped support only','profiles':profiles,'raw_sha256':sha(proof),'source_sha256':sha((HERE/'hash_one_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 fixture=SUPPORT.ROOT/'port/support/rusthashmap/testdata/hash_one.tsv';metadata=HERE/'hash_one_manifest.json'
 if a.check:
  if fixture.read_bytes()!=proof or json.loads(metadata.read_text())!=manifest:raise SystemExit('HashOne reference differs')
 else:fixture.write_bytes(proof);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print('Actual hash_one override dispatch matches both std profiles')
if __name__=='__main__':main()
