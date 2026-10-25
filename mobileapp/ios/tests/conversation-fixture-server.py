import base64
import urllib.parse
import argparse
import hashlib
import json
import http.server
import pathlib
import time
parser = argparse.ArgumentParser(description="Synthetic loopback document acceptance server")
parser.add_argument("--output", required=True, type=pathlib.Path)
parser.add_argument("--rtc-probe-port", type=int, default=0)
args = parser.parse_args()
if not 0 <= args.rtc_probe_port <= 65535:
 parser.error("rtc-probe-port must be 0..65535")
root = args.output
root.mkdir(parents=True, exist_ok=True)
hits=[]
contents = {}
attempts = {}
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args): pass
 def reply(self,status,body,mime='application/json',extra={}):
  self.send_response(status); self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(body)))
  for k,v in extra.items():self.send_header(k,v)
  self.end_headers();self.wfile.write(body)
 def do_POST(self):
  hits.append({'method':'POST','path':self.path,'auth':self.headers.get('Authorization')=='Bearer fixture-only'})
  req=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0')))); ref=req['reference']
  attempts[ref]=attempts.get(ref,0)+1
  if ref.endswith('recover') and attempts[ref]==1:self.reply(404,b'{"code":"document_unpublished"}');return
  if ref.endswith('redirect'):self.reply(302,b'',extra={'Location':'/beacon/redirect'});return
  if ref.endswith('unpublished'):self.reply(404,b'{"code":"document_unpublished"}');return
  if ref.endswith('old'):self.reply(404,b'Not found','text/plain');return
  if ref.endswith('denied'):self.reply(403,b'{}');return
  if ref.endswith('network'):self.reply(500,b'{}');return
  if ref.endswith('large'):self.reply(413,b'{}');return
  if ref.endswith('slow'):time.sleep(8)
  payload=html.encode();kind='html';mime='text/html';name='Report.html'
  if ref.endswith('.txt'):
   payload=('中文 selection 😀 — complete original file.\n' * 16000).encode();kind='text';mime='text/plain';name='Report.txt'
  elif ref.endswith('.png'):
   payload=base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j6XcAAAAASUVORK5CYII=');kind='image';mime='image/png';name='Report.png'
  elif ref.endswith('.pdf'):
   payload=pdf;kind='pdf';mime='application/pdf';name='Report.pdf'
  if ref.endswith('badimage'):payload=b'not an image';kind='image';mime='image/png';name='Report.png'
  sha=hashlib.sha256(payload).hexdigest()
  identifier=hashlib.sha256(ref.encode()).hexdigest()[:32]
  contents[identifier]=payload
  if ref.endswith('checksum'):sha='0'*64
  meta={'id':identifier,'name':name,'kind':kind,'media_type':mime,'bytes':len(payload),'sha256':sha,'session_id':'fixture','source':'/fixture/report.html','created_at':1792915200}
  self.reply(200,json.dumps(meta).encode())
 def do_GET(self):
  if self.path=='/stats':self.reply(200,json.dumps(hits).encode());return
  hits.append({'method':'GET','path':self.path,'auth':self.headers.get('Authorization')=='Bearer fixture-only'})
  if self.path.startswith('/api/artifacts/content'):
   identifier=urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query).get('id',[''])[0]
   self.reply(200,contents.get(identifier,b''),'application/octet-stream');return
  self.reply(200,b'BEACON','text/plain')
# A small valid PDF, generated without an image/font/library dependency.
objects=[b'<< /Type /Catalog /Pages 2 0 R >>',b'<< /Type /Pages /Kids [3 0 R] /Count 1 >>',b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>']
stream=b'BT /F1 24 Tf 72 720 Td (Conversation document preview) Tj ET'
objects.append(b'<< /Length '+str(len(stream)).encode()+b' >>\nstream\n'+stream+b'\nendstream')
pdf=b'%PDF-1.4\n'; offsets=[0]
for i,obj in enumerate(objects,1):
 offsets.append(len(pdf));pdf+=str(i).encode()+b' 0 obj\n'+obj+b'\nendobj\n'
xref=len(pdf);pdf+=b'xref\n0 6\n0000000000 65535 f \n'
for offset in offsets[1:]:pdf+=f'{offset:010d} 00000 n \n'.encode()
pdf+=f'trailer\n<< /Root 1 0 R /Size 6 >>\nstartxref\n{xref}\n%%EOF'.encode()
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
port=server.server_address[1]
html='''<!doctype html><html><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{font:16px -apple-system;padding:20px;color:#202124;background:#f6f7f9} h1{font-size:28px}table{border-collapse:collapse;background:white;width:100%}td,th{padding:14px;border-bottom:1px solid #ddd;text-align:left}button{font:inherit;padding:12px;border:0;border-radius:10px;color:white;background:#3158cd}</style><h1>本次更新 · Review</h1><p id="inline">LOADING</p><p>这是隔离数据。报告可滚动、选择文字，开启交互后可展开详情。</p><button onclick="this.textContent='已展开';document.getElementById('more').hidden=false">展开详情</button><p id="more" hidden>详细修改内容：文件预览与选字体验。</p><table><tr><th>功能</th><th>结果</th></tr><tr><td>交互内容</td><td>开启交互后可用</td></tr><tr><td>外部 HTTP 请求</td><td>已阻止</td></tr><tr><td>配对凭据</td><td>不进入文档</td></tr></table><div style="height:600px"></div><p>报告结束</p><script>document.getElementById('inline').textContent='INLINE_OK';if(UDPPORT>0){try{let p=new RTCPeerConnection({iceServers:[{urls:'stun:127.0.0.1:UDPPORT'}]});p.createDataChannel('synthetic');p.createOffer().then(o=>p.setLocalDescription(o));setTimeout(()=>p.close(),60000)}catch(e){}};let b='http://127.0.0.1:PORT/beacon/';fetch(b+'fetch').catch(()=>{});new Image().src=b+'image';try{new WebSocket('ws://127.0.0.1:PORT/beacon/socket')}catch(e){};window.open(b+'popup');let f=document.createElement('iframe');f.src=b+'frame';document.body.appendChild(f);</script></html>'''.replace('UDPPORT',str(args.rtc_probe_port)).replace('PORT',str(port))
(root/'fixture-server-port.txt').write_text(str(port))
server.serve_forever()
