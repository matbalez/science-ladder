import hashlib, os, pathlib, subprocess, tempfile, unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
class InstallerTest(unittest.TestCase):
    def exercise(self, corrupt=False, kernel='Darwin', arch='arm64'):
        with tempfile.TemporaryDirectory() as temp:
            p=pathlib.Path(temp)
            tools=p/'tools'; tools.mkdir()
            dest=p/'installed'; dest.mkdir()
            (dest/'sl').write_text('previous binary')
            (p/'binary').write_text('#!/bin/sh\necho "Science Ladder CLI test"\n')
            asset='sl_'+('darwin' if kernel=='Darwin' else 'linux')+'_'+('amd64' if arch=='x86_64' else 'arm64')
            digest='0'*64 if corrupt else hashlib.sha256((p/'binary').read_bytes()).hexdigest()
            (p/'checksums').write_text(digest+'  '+asset+'\n')
            (tools/'uname').write_text('#!/bin/sh\ncase "$1" in -s) echo "$TEST_KERNEL";; -m) echo "$TEST_ARCH";; esac\n')
            (tools/'curl').write_text('#!/bin/sh\nwhile [ "$#" -gt 0 ]; do case "$1" in https:*) remote="$1";; -o) shift; output="$1";; esac; shift; done\ncase "$remote" in */SHA256SUMS) cp "$TEST_ROOT/checksums" "$output";; *) cp "$TEST_ROOT/binary" "$output";; esac\n')
            for x in tools.iterdir(): x.chmod(0o755)
            run=subprocess.run(['sh',str(ROOT/'web/public/install.sh')],capture_output=True,text=True,env={**os.environ,'PATH':str(tools)+':'+os.environ['PATH'],'SL_INSTALL_DIR':str(dest),'TEST_ROOT':str(p),'TEST_KERNEL':kernel,'TEST_ARCH':arch})
            if corrupt:
                self.assertNotEqual(run.returncode,0); self.assertIn('Checksum mismatch',run.stderr)
                self.assertEqual((dest/'sl').read_text(),'previous binary')
            else:
                self.assertEqual(run.returncode,0,run.stderr); self.assertIn('Science Ladder CLI test',run.stdout)
                self.assertTrue(os.access(dest/'sl',os.X_OK))
    def test_installs_supported_platforms(self):
        for kernel in ['Darwin','Linux']:
            for arch in ['arm64','x86_64']:
                with self.subTest(kernel=kernel,arch=arch): self.exercise(kernel=kernel,arch=arch)
    def test_checksum_failure_preserves_existing_install(self): self.exercise(corrupt=True)
