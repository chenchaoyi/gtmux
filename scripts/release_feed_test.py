"""Synthetic signing, bootstrap and publication failure tests; no production credentials."""
import base64
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT=Path(__file__).resolve().parent.parent
spec=importlib.util.spec_from_file_location('release_publisher',ROOT/'scripts/publish-release-feed.py')
publisher=importlib.util.module_from_spec(spec);spec.loader.exec_module(publisher)


class ReleaseFeedTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp=tempfile.TemporaryDirectory(prefix='gtmux-signing-test-')
        cls.root=Path(cls.temp.name);cls.key=cls.root/'private.pem';cls.public=cls.root/'internal/releasefeed/public.pem';cls.public.parent.mkdir(parents=True)
        subprocess.run(['openssl','genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:3072','-out',str(cls.key)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        subprocess.run(['openssl','pkey','-in',str(cls.key),'-pubout','-out',str(cls.public)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

    @classmethod
    def tearDownClass(cls): cls.temp.cleanup()

    def setUp(self):
        self.workspace=tempfile.TemporaryDirectory(dir=self.root);self.addCleanup(self.workspace.cleanup)
        self.directory=Path(self.workspace.name);self.installer=self.directory/'source.sh';self.installer.write_text("#!/bin/bash\nprintf 'VERIFIED_INSTALLER:%s' \"$GTMUX_VERSION\"\n")
        self.metadata={'tag_name':'v1.2.3','draft':False,'prerelease':False,'published_at':'2026-01-01T00:00:00Z','assets':[]}
        sums=[]
        for name in publisher.names('1.2.3')[:5]:
            path=self.directory/name;path.write_bytes(name.encode());sha=hashlib.sha256(path.read_bytes()).hexdigest()
            if name.endswith('.tar.gz'):sums.append(sha+'  '+name)
        (self.directory/'SHASUMS256.txt').write_text('\n'.join(sums)+'\n')
        for name in publisher.names('1.2.3'):
            path=self.directory/name;self.metadata['assets'].append({'name':name,'size':path.stat().st_size,'digest':'sha256:'+publisher.digest(path)})

    def prepare(self):
        with patch.object(publisher,'ROOT',self.root):
            return publisher.prepare('1.2.3','a'*40,'b'*40,self.directory,self.metadata,self.installer,self.key)

    def test_bytes_and_source_scope(self):
        m=self.prepare();self.assertEqual(m['source_commit'],'a'*40);self.assertEqual(m['installer_source'],'b'*40)
        self.assertEqual(set(m['assets']),set(publisher.names('1.2.3')+['install.sh']))
        envelope=json.loads((self.directory/'manifest.json').read_bytes())
        self.assertEqual(json.loads(base64.b64decode(envelope['payload'])),m)

    def test_wrong_github_digest_and_incomplete_release_stop(self):
        self.metadata['assets'][0]['digest']='sha256:'+'0'*64
        with self.assertRaises(ValueError):self.prepare()
        self.metadata['draft']=True
        with self.assertRaises(ValueError):self.prepare()

    def bootstrap(self, mode='valid', pin=''):
        self.prepare()
        script=(ROOT/'bootstrap.sh').read_text().replace((ROOT/'internal/releasefeed/public.pem').read_text().strip(),self.public.read_text().strip())
        (self.directory/'bootstrap.sh').write_text(script)
        bin_dir=self.directory/'bin';bin_dir.mkdir(exist_ok=True)
        stub=bin_dir/'curl';stub.write_text('''#!/usr/bin/env python3
import sys,os
from pathlib import Path
args=sys.argv[1:];url=next(x for x in args if x.startswith('https://'));root=Path(os.environ['FIXTURE_ROOT']);mode=os.environ['FIXTURE_MODE']
with (root/'requests').open('a') as f:f.write(url+'\\n')
if mode=='missing-pin' and '/v1.2.4/' in url:sys.exit(22)
if mode=='unavailable' or mode=='first-unavailable' and 'downloads.ccy.dev/' in url:sys.exit(22)
out=Path(args[args.index('-o')+1]);name=url.rsplit('/',1)[1]
if name in ('stable.json','manifest.json'):
 body=(root/'manifest.json').read_bytes()
 if mode=='invalid':body=body.replace(b'"signature":"',b'"signature":"A',1)
else:body=(root/name).read_bytes()
if mode=='bad-installer' and name=='install.sh':body+=b'modified'
out.write_bytes(body)
''');stub.chmod(0o755)
        env={**os.environ,'PATH':str(bin_dir)+os.pathsep+os.environ['PATH'],'FIXTURE_ROOT':str(self.directory),'FIXTURE_MODE':mode,'GTMUX_VERSION':pin}
        return subprocess.run(['bash',str(self.directory/'bootstrap.sh')],env=env,text=True,capture_output=True)

    def test_bootstrap_verifies_and_pins_exact_installer(self):
        for mode in ('valid','first-unavailable'):
            with self.subTest(mode=mode):
                result=self.bootstrap(mode);self.assertEqual(result.returncode,0,result.stderr);self.assertIn('VERIFIED_INSTALLER:v1.2.3',result.stdout)

    def test_bad_signature_hash_and_pin_never_execute(self):
        for mode,pin in [('invalid',''),('bad-installer',''),('valid','v1.2.4'),('unavailable','')]:
            with self.subTest(mode=mode,pin=pin):
                result=self.bootstrap(mode,pin);self.assertNotEqual(result.returncode,0);self.assertNotIn('VERIFIED_INSTALLER:',result.stdout)

    def test_unmirrored_pin_uses_authenticated_current_installer(self):
        result=self.bootstrap('missing-pin','v1.2.4')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIn('VERIFIED_INSTALLER:v1.2.4',result.stdout)

    def test_publisher_never_sends_credential_to_other_host(self):
        with self.assertRaises(ValueError):publisher.transfer('https://example.com/publish/v1.2.3/install.sh',self.installer,'synthetic-token','0'*64)

    def test_failed_or_unnotarized_builds_are_not_publishable(self):
        run={'id':1,'head_branch':'v1.2.3','head_sha':'a'*40,'event':'push','path':'.github/workflows/release.yml'}
        for steps in ([],[{'name':'verify the app is notarized and stapled (tag)','conclusion':'skipped'}]):
            jobs={'jobs':[{'name':'goreleaser','conclusion':'success'},{'name':'menu-bar app (macOS)','conclusion':'success','steps':steps}]}
            with patch.object(publisher,'gh',side_effect=[run,jobs]),self.assertRaises(ValueError):publisher.require_builds('a'*40,1,'1.2.3')
        jobs={'jobs':[{'name':'goreleaser','conclusion':'success'},{'name':'menu-bar app (macOS)','conclusion':'success','steps':[{'name':'verify the app is notarized and stapled (tag)','conclusion':'success'}]}]}
        with patch.object(publisher,'gh',side_effect=[run,jobs]):self.assertEqual(publisher.require_builds('a'*40,1,'1.2.3'),1)


class PublicationRunSelectionTest(unittest.TestCase):
 def test_publication_and_readback_identify_the_client_without_leaking_auth(self):
    class Reply(io.BytesIO):
        status=200
    with tempfile.TemporaryDirectory(prefix='gtmux-publisher-http-') as temp:
        source=Path(temp)/'artifact';source.write_bytes(b'synthetic archive')
        requests=[]
        def opened(request,timeout):
            requests.append(request)
            return Reply(b'synthetic archive')
        with patch.object(publisher.HTTP,'open',side_effect=opened):
            publisher.transfer('https://gtmux-downloads.pages.dev/publish/v1.2.3/install.sh',source,'synthetic-token','a'*64)
            publisher.transfer('https://downloads.ccy.dev/gtmux/v1.2.3/install.sh',Path(temp)/'readback')
        self.assertEqual([request.get_header('User-agent') for request in requests],['gtmux-release-publisher/1']*2)
        self.assertEqual(requests[0].get_header('Authorization'),'Bearer synthetic-token')
        self.assertFalse(requests[1].has_header('Authorization'))

 def test_latest_failed_build_cannot_reuse_an_old_pass(self):
    run={'id':2,'head_branch':'v1.2.3','head_sha':'a'*40,'event':'push','path':'.github/workflows/release.yml'}
    with patch.object(publisher,'gh',side_effect=[{'workflow_runs':[run,{**run,'id':1}]},{'jobs':[]}]) as api:
        try: publisher.require_builds('a'*40,None,'1.2.3')
        except ValueError: pass
        else: raise AssertionError('An older successful build must not be borrowed')
        assert api.call_count==2
