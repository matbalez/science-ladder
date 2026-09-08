#!/usr/bin/env python3
"""MIT. Build operator-provisioned Linux proof assets in a new work directory.
Requires Python 3.12+, zstd, Git and Docker. This is not a solver installation step.
"""
import hashlib,json,os,shutil,subprocess,sys,tarfile,urllib.request
from pathlib import Path
HERE=Path(__file__).resolve().parent
NATIVE='ghcr.io/matbalez/science-ladder-native@sha256:5167ee2dc5e3241824ed49ee9b5df28676e2c2b56983d5d5600aec32af4ae22b'
TOOLS=os.getenv('SL_PROOF_TOOLS_IMAGE','')
TOOLS_BASE='python@sha256:ed86c82274b3c69b52fb5820f358f0bd7df0b603332063cb5c6e32bd220c3e6e'
LEAN='4.33.1'
ARCHIVE_SHA='890afd185370f85666025b883914ab4f4b339136f8c96167b69cfb62aecaf235'
PINS={'comparator':('leanprover/comparator','c0c5a52d2aff92b457c3e5ed4a68c1ebc5795809'),'lean4export':('leanprover/lean4export','15f6055e299ad5b89345e533cc2192f4cc00f659'),'drat-trim':('marijnheule/drat-trim','2e3b2dc0ecf938addbd779d42877b6ed69d9a985')}
def run(args,**kwargs):subprocess.run(args,check=True,**kwargs)
def digest(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def main():
 if len(sys.argv)!=2:raise SystemExit('usage: python3 tools/proof/build.py NEW_DIRECTORY')
 work=Path(sys.argv[1]).resolve();work.mkdir(parents=True,exist_ok=False)
 tools=TOOLS
 if not tools:
  run(['docker','build','--platform','linux/amd64','--build-arg','BASE_IMAGE='+TOOLS_BASE,'--build-arg','DEBIAN_SNAPSHOT=20260801T000000Z','-f',str(HERE.parent.parent/'internal/runner/assets/platform-tools.Dockerfile'),'-t','science-ladder-proof-builder-local',str(HERE.parent.parent/'internal/runner/assets')])
  tools=subprocess.check_output(['docker','image','inspect','--format','{{.Id}}','science-ladder-proof-builder-local'],text=True).strip()
 def docker(image,entry,args,env=(),cwd='/workspace'):
  run(['docker','run','--rm','--platform','linux/amd64','--mount',f'type=bind,src={work},dst=/workspace','-w',cwd,*[x for v in env for x in ('-e',v)],'--entrypoint',entry,image,*args])
 url=f'https://github.com/leanprover/lean4/releases/download/v{LEAN}/lean-{LEAN}-linux.tar.zst'
 archive=work/'lean.tar.zst'
 with urllib.request.urlopen(url,timeout=120) as response,archive.open('wb') as out:shutil.copyfileobj(response,out)
 if digest(archive)!=ARCHIVE_SHA:raise SystemExit('Lean archive digest mismatch')
 with subprocess.Popen(['zstd','-d','-c',str(archive)],stdout=subprocess.PIPE) as unpack:
  with tarfile.open(fileobj=unpack.stdout,mode='r|') as tf:tf.extractall(work,filter='data')
  if unpack.wait()!=0:raise SystemExit('Lean decompression failed')
 for name,(repo,commit) in PINS.items():
  target=work/name;run(['git','init',str(target)]);run(['git','-C',str(target),'fetch','--depth=1','https://github.com/'+repo,commit]);run(['git','-C',str(target),'checkout','--detach','FETCH_HEAD'])
 shutil.copy2(HERE/'lean/SLCheck.lean',work/'comparator/SLCheck.lean')
 (work/'comparator/lakefile.toml').write_text('name = "Comparator"\nversion = "0.1.0"\n[[lean_lib]]\nname = "Comparator"\n[[require]]\nname = "lean4export"\npath = "../lean4export"\n[[lean_exe]]\nname = "sl-lean-check"\nroot = "SLCheck"\n')
 lean=f'/workspace/lean-{LEAN}-linux/bin'
 docker(NATIVE,lean+'/lake',['build','sl-lean-check'],env=(f'PATH={lean}:/usr/bin:/bin',),cwd='/workspace/comparator')
 docker(NATIVE,lean+'/lake',['build','lean4export'],env=(f'PATH={lean}:/usr/bin:/bin',),cwd='/workspace/lean4export')
 gcc="import os; r='/opt/sl-private/toolchain'; [os.symlink('usr/'+x,r+'/'+x) for x in ('lib','lib64','bin','sbin') if not os.path.lexists(r+'/'+x)]; os.environ['LD_LIBRARY_PATH']=r+'/usr/lib/x86_64-linux-gnu'; os.environ['PATH']=r+'/usr/bin:/usr/bin:/bin'; os.execv(r+'/usr/bin/gcc',['gcc','--sysroot='+r,'-O2','/workspace/drat-trim/drat-trim.c','-o','/workspace/drat-trim/drat-trim-pinned'])"
 docker(NATIVE,'/usr/local/bin/python3',['-c',gcc])
 shutil.copytree(HERE/'fixtures',work/'fixtures');shutil.copy2(HERE/'test-lean.sh',work/'test-lean.sh');shutil.copy2(HERE/'test-drat.sh',work/'test-drat.sh')
 # Use the native image's operator-accessible shell; checker /usr lacks a shell.
 docker(NATIVE,'/opt/sl-private/toolchain/usr/bin/dash',['/workspace/test-lean.sh'])
 docker(NATIVE,'/opt/sl-private/toolchain/usr/bin/dash',['/workspace/test-drat.sh','/workspace/drat-trim/drat-trim-pinned','/workspace/fixtures'])
 assets=work/'assets';candidate=assets/'lean';(candidate/'bin').mkdir(parents=True)
 # Only the elaborator, imported Lean libraries and exporter are needed in-guest.
 # Native C/C++ and Rust builds use the separately reviewed platform toolchain.
 (candidate/'lean/bin').mkdir(parents=True);shutil.copy2(work/f'lean-{LEAN}-linux/bin/lean',candidate/'lean/bin/lean')
 shutil.copytree(work/f'lean-{LEAN}-linux/lib/lean',candidate/'lean/lib/lean',symlinks=True)
 for f in (candidate/'lean/lib/lean').glob('*.a'):f.unlink()
 shutil.copy2(work/'lean4export/.lake/build/bin/lean4export',candidate/'bin/lean4export')
 checker=assets/'proof-tools';checker.mkdir()
 for src,name in [('comparator/.lake/build/bin/sl-lean-check','sl-lean-check'),('drat-trim/drat-trim-pinned','drat-trim'),('fixtures/Reference.ndjson','reference.ndjson'),('fixtures/policy.json','policy.json')]:shutil.copy2(work/src,checker/name)
 for folder in (candidate,checker):
  notices=folder/'notices';notices.mkdir()
  for upstream in PINS:
   f=next(f for f in (work/upstream).glob('LICENSE*') if f.is_file());shutil.copy2(f,notices/(upstream+'-LICENSE'))
  for name in ('LICENSE','LICENSES'):shutil.copy2(work/f'lean-{LEAN}-linux'/name,notices/('lean-'+name))
 for name,text in [('public-data','public asset visible'),('hidden-data','hidden asset canary')]:
  p=assets/name;p.mkdir();(p/'canary.txt').write_text(text)
 result={}
 for folder in sorted(assets.iterdir()):
  inventory=[]
  for f in sorted(folder.rglob('*')):
   if f.is_symlink():inventory.append({'path':str(f.relative_to(folder)),'link':str(f.readlink())})
   elif f.is_file():inventory.append({'path':str(f.relative_to(folder)),'sha256':digest(f),'size':f.stat().st_size})
  (assets/(folder.name+'-files.json')).write_text(json.dumps(inventory,separators=(',',':'))+'\n')
  docker(tools,'/usr/bin/mksquashfs',[f'/workspace/assets/{folder.name}',f'/workspace/{folder.name}.squashfs','-noappend','-all-root','-comp','zstd','-processors','2','-no-progress'])
  image=work/(folder.name+'.squashfs');result[folder.name]={'digest':'sha256:'+digest(image),'size':image.stat().st_size}
 (work/'asset-bytes.json').write_text(json.dumps(result,indent=2)+'\n')
 print('Built and locally tested proof assets. Advisory review, signed host conformance and enrollment are separate operator steps.')
if __name__=='__main__':main()
