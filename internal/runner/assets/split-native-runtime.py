"""MIT. Keep compiler tools outside the checker filesystem authority.
The candidate receives a read-only bind of the full toolchain /usr. The checker
receives only the explicit Python/numerical dependency closure. Every package
and installed file remains inventoried, including candidate-only components.
"""
import hashlib,json,os,re,shutil,subprocess
from pathlib import Path
fmt='${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\t${Depends}\t${Pre-Depends}\t${Provides}\n'
rows={};providers={}
for line in subprocess.check_output(['dpkg-query','-W','-f='+fmt],text=True).splitlines():
    name,version,source,sv,deps,pre,provides=line.split('\t');name=name.split(':')[0]
    for value in provides.split(','):
        if value.strip():providers[value.strip().split()[0]]=name
    rows[name]={'ecosystem':'Debian','name':name,'version':version,'sourceName':source,'sourceVersion':sv,'dependencies':','.join((deps,pre))}
needed=set()
def require(name):
    if name in needed:return
    if name not in rows:raise ValueError('missing checker dependency '+name)
    needed.add(name)
    for dependency in rows[name]['dependencies'].split(','):
        if not dependency.strip():continue
        options=[re.split(r'\s|:',s.strip())[0] for s in dependency.split('|')]
        installed=next((x if x in rows else providers.get(x) for x in options if x in rows or x in providers),None)
        if installed is None:raise ValueError('unresolved checker dependency '+dependency)
        require(installed)
for name in ('python3','python3-numpy','python3-scipy'):require(name)
# These are installation-time dependency tools, not Python runtime libraries.
needed.difference_update({'debconf','dpkg','tar'})
private=Path('/opt/sl-private');private.mkdir(mode=0o700)
toolchain=private/'toolchain';toolchain.mkdir(mode=0o755)
checker=private/'checker-usr';checker.mkdir(mode=0o755)
for name in sorted(needed):
    for filename in subprocess.check_output(['/usr/bin/dpkg-query','-L',name],text=True).splitlines():
        # Source paths in the package DB precede merged-/usr normalization.
        if filename.startswith(('/lib/','/lib64/','/bin/','/sbin/')):filename='/usr'+filename
        if not filename.startswith('/usr/') or filename.startswith(('/usr/share/doc/','/usr/share/man/','/usr/share/lintian/')):continue
        src=Path(filename);dest=checker/filename.removeprefix('/usr/')
        if not src.exists() and not src.is_symlink():continue
        if src.is_dir() and not src.is_symlink():dest.mkdir(parents=True,exist_ok=True);continue
        dest.parent.mkdir(parents=True,exist_ok=True)
        if dest.exists() or dest.is_symlink():continue
        if src.is_symlink():dest.symlink_to(os.readlink(src))
        else:shutil.copy2(src,dest)
# ldconfig/update-alternatives generate aliases absent from package file lists.
# Retain an alias only when its resolved library is in the checker closure.
for alias in Path('/usr').rglob('*'):
    if not alias.is_symlink():continue
    resolved=alias.resolve()
    if not resolved.is_relative_to('/usr'):continue
    retained=checker/resolved.relative_to('/usr')
    dest=checker/alias.relative_to('/usr')
    if retained.is_file() and not dest.exists() and not dest.is_symlink():
        dest.parent.mkdir(parents=True,exist_ok=True);dest.symlink_to(os.readlink(alias))
# Root broker launches this static boundary program; Python alias is common to
# the challenge protocol. Compiler binaries do not get aliases in checker /usr.
(checker/'local/bin').mkdir(parents=True,exist_ok=True)
shutil.copy2('/usr/local/bin/sl-candidate-sandbox',checker/'local/bin/sl-candidate-sandbox')
if not (checker/'local/bin/python3').is_symlink():(checker/'local/bin/python3').symlink_to('/usr/bin/python3')
packages=[]
for name,row in sorted(rows.items()):
    row.pop('dependencies');row['executionDomain']='checker' if name in needed else 'candidate-only';packages.append(row)
files=[]
for root,domain in ((checker,'checker'),(Path('/usr'),'candidate')):
    for path in sorted(root.rglob('*')):
        finalPath=Path('/usr')/path.relative_to(root) if domain=='checker' else toolchain/'usr'/path.relative_to(root)
        if path.is_symlink():files.append({'path':str(finalPath),'domain':domain,'link':os.readlink(path)})
        elif path.is_file():files.append({'path':str(finalPath),'domain':domain,'digest':'sha256:'+hashlib.sha256(path.read_bytes()).hexdigest()})
doc={'version':'native-domain-inventory/v1','checkerPackages':sorted(needed),'packages':packages,'files':files}
(checker/'share/science-ladder').mkdir(parents=True,exist_ok=True)
(checker/'share/science-ladder/runtime-components.json').write_text(json.dumps(doc,separators=(',',':'))+'\n')
print('Checker dependency packages:',','.join(sorted(needed)),flush=True)

shutil.move('/usr',toolchain/'usr')
checker.rename('/usr')
