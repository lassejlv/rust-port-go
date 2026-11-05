# SPDX-License-Identifier: MIT OR Apache-2.0
"""Mechanical selection of the actual portable used-path helpers for PE replay.

No SourceFile/SourceMap/session types or replacement algorithms are introduced.
The complete three production helper files and existing identical declaration
blocks below are compiled as a standalone CLI, before the full Windows span
host adapter has completed validation. Selected bytes are bound in evidence.
"""
import hashlib,re
from pathlib import Path
ROOT=Path(__file__).absolute().parents[3]
SRC=ROOT/'port/rust/compiler/rustc_span/src'
def function(text,name):
 start=text.index('func '+name+'(');end=text.find('\nfunc ',start+1)
 return text[start:] if end<0 else text[start:end]
def sources(dest):
 result={}
 def save(name,text):
  (dest/name).write_text(text.replace('package rustcspan','package main',1));result[name]=hashlib.sha256(text.encode()).hexdigest()
 for name in ['source_map_path.go','source_map_path_traits.go','symbol_unicode.go']:save(name,(SRC/name).read_text())
 lib=(SRC/'lib.go').read_text();save('option.go',lib[:lib.index('type BytePos')])
 path=(SRC/'source_map_path_oracle_test.go').read_text();path=path[:path.index('func TestSourceMapUnixPathNativeOracle(')]
 for unused in ['bufio','compress/gzip','os','testing']:path=path.replace('\t"'+unused+'"\n','')
 save('oracle.go',path)
 formatter=(SRC/'source_map_format_oracle_test.go').read_text();strings=(SRC/'source_map_oracle_test.go').read_text()
 save('format_options.go','package main\nimport "github.com/lassejlv/rust-port-go/port/support/rustfmt"\n'+function(formatter,'sourceMapFormatOptions')+'\n'+function((SRC/'fatal_error_format_test.go').read_text(),'fatalFormatOptions'))
 save('unhex.go','package main\nimport "encoding/hex"\n'+function(strings,'sourceMapOracleString'))
 main=(Path(__file__).parent/'main.go.txt').read_text();save('main.go',main)
 return result
