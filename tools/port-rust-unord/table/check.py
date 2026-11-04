#!/usr/bin/env python3
"""Recheck the retained original-source backing proofs without rewriting them."""
import os,pathlib,shutil,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def main():
 for name in ['generate.py','ownership_generate.py','abort_generate.py','hash_one_generate.py','fx_generate.py','scalar_generate.py','layout_generate.py','format_generate.py','iterator_generate.py']:
  command=[sys.executable,str(HERE/name),'--check']
  if name=='generate.py':command.append('--skip-go')
  subprocess.run(command,cwd=ROOT,check=True)
 selected=os.environ.get('FERRUGO_GO');go=shutil.which('go'if selected is None else selected)
 if selected is not None and not go:raise SystemExit('Explicit FERRUGO_GO does not resolve')
 if not go:go=str(ROOT/'.toolchain/go/bin/go')
 subprocess.run([go,'test','./port/support/rusthashmap','-count=1'],cwd=ROOT,check=True)
if __name__=='__main__':main()
