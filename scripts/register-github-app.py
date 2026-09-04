#!/usr/bin/env python3
"""One-use loopback GitHub App registration; never prints returned credentials."""
import argparse
import html
import http.server
import json
import os
from pathlib import Path
import secrets
import time
import urllib.parse
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--env-file', required=True, type=Path)
parser.add_argument('--origin', required=True)
parser.add_argument('--name', default='Science Ladder')
args = parser.parse_args()
if not args.origin.startswith('https://'):
    parser.error('The hosted origin must use HTTPS')
target = args.env_file.absolute()
if target.is_symlink():
    parser.error('Refusing a symlink destination')
state = secrets.token_urlsafe(32)
started = time.monotonic()
done = False

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def send_html(self, status, body):
        data = ('<!doctype html><html lang="en"><meta charset="utf-8"><title>Science Ladder setup</title>'
                '<meta name="viewport" content="width=device-width"><style>body{font:18px system-ui;'
                'background:#111817;color:#edf7ef;max-width:700px;margin:12vh auto;padding:24px}'
                'button{font:inherit;padding:15px 24px;background:#cbff70;border:0;border-radius:7px;cursor:pointer}'
                '</style>'+body+'</html>').encode()
        self.send_response(status)
        self.send_header('Content-Type','text/html; charset=utf-8')
        self.send_header('Cache-Control','no-store')
        self.send_header('Referrer-Policy','no-referrer')
        self.send_header('X-Content-Type-Options','nosniff')
        self.send_header('Content-Length',str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        global done
        if self.headers.get('Host') != '127.0.0.1:'+str(self.server.server_port):
            return self.send_html(403,'Invalid host.')
        parsed = urllib.parse.urlparse(self.path)
        query = urllib.parse.parse_qs(parsed.query)
        if time.monotonic()-started > 3500 or done:
            return self.send_html(410,'This setup session has ended.')
        if parsed.path == '/':
            manifest = {
                'name':args.name,
                'url':args.origin,
                'redirect_url':f'http://127.0.0.1:{self.server.server_port}/callback',
                'callback_urls':[args.origin+'/v1/auth/github/callback'],
                'setup_url':args.origin+'/account',
                'description':'Open scientific challenges with reproducible verification and shared frontier progress.',
                'hook_attributes':{'url':args.origin+'/v1/webhooks/github','active':True},
                'public':True,
                'default_permissions':{'contents':'read','metadata':'read'},
                'default_events':['push'],
            }
            form = ('<h1>Connect Science Ladder to GitHub</h1><p>This registers an app owned by your GitHub account. '
                    'It requests read access to repository contents and metadata, only for repositories you select when installing it. '
                    'It does not request write or organization permissions.</p><p>The app’s credentials will be saved directly to your '
                    'private local configuration.</p><form method="post" action="https://github.com/settings/apps/new?state='+html.escape(state)+'">'
                    '<input type="hidden" name="manifest" value="'+html.escape(json.dumps(manifest),quote=True)+'">'
                    '<button type="submit">Review on GitHub</button></form>')
            return self.send_html(200,form)
        if parsed.path != '/callback' or not secrets.compare_digest(query.get('state',[''])[0],state):
            return self.send_html(403,'Invalid registration state.')
        code=query.get('code',[''])[0]
        if not code or len(code)>200 or not all(c.isalnum() or c in '-_' for c in code):
            return self.send_html(400,'Missing registration code.')
        try:
            request=urllib.request.Request('https://api.github.com/app-manifests/'+code+'/conversions',
                data=b'',headers={'Accept':'application/vnd.github+json','User-Agent':'science-ladder-setup',
                                  'X-GitHub-Api-Version':'2022-11-28'},method='POST')
            with urllib.request.urlopen(request,timeout=30) as response:
                app=json.load(response)
            values={'GITHUB_APP_ID':str(app['id']),'GITHUB_APP_SLUG':app['slug'],
                    'GITHUB_CLIENT_ID':app['client_id'],'GITHUB_CLIENT_SECRET':app['client_secret'],
                    'GITHUB_APP_PRIVATE_KEY_PEM':app['pem'],'GITHUB_WEBHOOK_SECRET':app['webhook_secret']}
            old=target.read_text() if target.exists() else ''
            lines=[l for l in old.splitlines() if l.split('=',1)[0] not in values]
            lines += [k+'='+json.dumps(v) for k,v in values.items()]
            tmp=target.with_name(target.name+'.tmp-'+secrets.token_hex(5))
            fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
            with os.fdopen(fd,'w') as f:
                f.write('\n'.join(lines)+'\n');f.flush();os.fsync(f.fileno())
            os.replace(tmp,target)
            done=True
            print(json.dumps({'registered':True,'app_id':app['id'],'slug':app['slug'],'saved':str(target)}),flush=True)
            return self.send_html(200,'<h1>GitHub App registered</h1><p>The credentials were saved securely. '
                                  'You can return to Codex while deployment continues.</p><p><a style="color:#cbff70" href="'+
                                  html.escape(app['html_url'])+'/installations/new">Install on selected repositories</a></p>')
        except Exception as exc:
            print(json.dumps({'registered':False,'error_type':type(exc).__name__}),flush=True)
            return self.send_html(502,'<h1>Setup could not finish</h1><p>No credentials were displayed. Return to Codex for recovery.</p>')

server=http.server.HTTPServer(('127.0.0.1',0),Handler)
print(json.dumps({'setup_url':f'http://127.0.0.1:{server.server_port}/'}),flush=True)
server.serve_forever()
