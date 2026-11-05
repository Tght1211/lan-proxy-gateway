import json,socket,subprocess,sys
from harness import require_container
require_container()
ip=sys.argv[1]
def run(*args):
    p=subprocess.run(args,text=True,capture_output=True,timeout=15)
    assert p.returncode==0,(args,p.stdout,p.stderr)
    return p.stdout
run('ip','route','add','198.18.0.0/16','via',ip)
fake=run('dig','+short','+time=2','+tries=1','@'+ip,'audit.invalid','A').strip()
assert fake.startswith('198.18.'),fake
print('PASS client DNS port 53 redirects to gateway 1053:',fake,flush=True)
body=run('curl','--noproxy','*','-fsS','--max-time','8','http://'+fake+':18080/')
assert body=='linux-audit-origin-ok',body
print('PASS client TCP -> iptables REDIRECT -> original destination -> SOCKS5 -> origin',flush=True)
s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
s.settimeout(5)
s.connect((fake,18083))
s.send(b'linux-udp-audit')
try:
    payload=s.recv(4096)
    assert payload==b'linux-udp-audit',payload
    print('PASS client fake-IP UDP echo',flush=True)
except Exception as exc:
    print('FAIL client fake-IP UDP echo:',repr(exc),flush=True)
    raise
