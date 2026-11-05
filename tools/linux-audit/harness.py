import http.server, json, os, pathlib, select, socket, socketserver, subprocess, sys, threading, time, urllib.request
BIN = None
checks=[]
upstream=[]
def run(*args, input=None, ok=True):
    p=subprocess.run(args,input=input,text=True,capture_output=True,timeout=25)
    if ok and p.returncode: raise RuntimeError(f'{args}: {p.stderr}\n{p.stdout}')
    return p

def cli(*args, **kwargs): return run(BIN,*args,**kwargs)
def check(name, condition=True):
    assert condition,name
    checks.append(name)
    print('PASS',name,flush=True)
def api(path):
    token=pathlib.Path('/root/.config/lan-proxy-gateway/api-token').read_text().strip()
    r=urllib.request.Request('http://127.0.0.1:19090/api/'+path,headers={'Authorization':'Bearer '+token})
    return json.load(urllib.request.urlopen(r,timeout=5))
def curl(url, *args):
    return run('curl','--silent','--show-error','--max-time','6','--noproxy','','--proxy','http://127.0.0.1:17894',*args,url)
def rules(items): cli('routing','set','--rules-json',json.dumps(items))
class Origin(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        data=b'linux-audit-origin-ok'
        self.send_response(200); self.send_header('Content-Length',str(len(data))); self.end_headers(); self.wfile.write(data)
    def log_message(self,*args): pass

def tunnel(client):
    with socket.create_connection(('127.0.0.1',18080),timeout=3) as origin:
        client.settimeout(5)
        for _ in range(50):
            ready,_,_=select.select([client,origin],[],[],3)
            if not ready: return
            for src in ready:
                data=src.recv(65536)
                if not data: return
                (origin if src is client else client).sendall(data)
class Connect(socketserver.StreamRequestHandler):
    def handle(self):
        try:
            first=self.rfile.readline().decode().strip()
            if not first: return
            while True:
                header = self.rfile.readline()
                if not header or not header.strip():
                    break
            upstream.append(('http',first))
            self.wfile.write(b'HTTP/1.1 200 Connection Established\r\n\r\n'); self.wfile.flush()
            tunnel(self.request)
        except (OSError,ValueError): pass
class Socks(socketserver.StreamRequestHandler):
    def handle(self):
        try:
            head=self.rfile.read(2)
            if len(head)!=2: return
            self.rfile.read(head[1]); self.wfile.write(b'\x05\x00'); self.wfile.flush()
            head=self.rfile.read(4)
            if head[3]==3: host=self.rfile.read(self.rfile.read(1)[0]).decode()
            elif head[3]==1: host=socket.inet_ntoa(self.rfile.read(4))
            else: return
            port=int.from_bytes(self.rfile.read(2),'big')
            upstream.append(('socks5',f'{host}:{port}'))
            self.wfile.write(b'\x05\x00\x00\x01\x7f\x00\x00\x01\x46\xa0'); self.wfile.flush()
            tunnel(self.request)
        except (OSError,ValueError,IndexError): pass
class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address=True
    daemon_threads=True
def require_container():
    if not pathlib.Path('/.dockerenv').exists() or os.environ.get('GATEWAY_AUDIT_CONTAINER') != '1':
        raise RuntimeError('Run this test with tools/linux-audit/run.py inside the audit container')


def initialize(binary):
    global BIN
    require_container()
    BIN = binary
    for server in [
        http.server.ThreadingHTTPServer(('127.0.0.1', 18080), Origin),
        Server(('127.0.0.1', 18081), Connect),
        Server(('127.0.0.1', 18082), Socks),
    ]:
        threading.Thread(target=server.serve_forever, daemon=True).start()
