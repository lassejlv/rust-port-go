import importlib.util,pathlib,unittest
spec=importlib.util.spec_from_file_location('ambient_probe',pathlib.Path(__file__).with_name('generate.py'));probe=importlib.util.module_from_spec(spec);spec.loader.exec_module(probe)
class Matrix(unittest.TestCase):
 def test_every_failure_shape_has_all_ambient_and_profile_controls(self):
  cases=probe.cases();self.assertEqual(len(cases),1344);self.assertEqual(len({r['id']for r in cases}),len(cases))
  for kind,value,session in [('span','bad_index',False),('span','bad_index',True),('ord','bad_index',True),('data','needs_interner',False)]:
   for debug in [False,True]:
    for overflow in [False,True]:
     selected=[r for r in cases if(r['kind'],r['value'],r['session'],r['debug_assertions'],r['overflow_checks'])==(kind,value,session,debug,overflow)]
     self.assertEqual({r['ambient']for r in selected},{'normal','unwinding','processing_panic'});self.assertEqual({r['fail']for r in selected},{-1,0})
if __name__=='__main__':unittest.main()
