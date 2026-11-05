import json
import pathlib
import socket
import sys
import threading
import time

from harness import api, check, checks, cli, curl, initialize, rules, run, upstream

initialize(sys.argv[1])
try:
    print('VERSION',cli('--version').stdout.strip(),flush=True)
    cli('--help'); cli('init')
    status=json.loads(cli('status','--json').stdout)
    check('initialize + status JSON',status['configured'] and not status['running'])
    check('network detection',bool(status['gateway']['LocalIP']))
    rules([])
    cli('http-proxy','set',input=json.dumps({'enabled':True,'port':17894,'auth':'none','username':''}))
    # Exercise real IP-forward setup and restoration inside this container namespace.
    pathlib.Path('/proc/sys/net/ipv4/ip_forward').write_text('0')
    cli('start')
    check('daemon start + API',json.loads(cli('status','--json').stdout)['running'] and api('stats')['schema_version']>=3)
    check('enable forwarding',pathlib.Path('/proc/sys/net/ipv4/ip_forward').read_text().strip()=='1')
    check('real iptables installed','lan-proxy-gateway' in run('iptables-save').stdout)
    check('HTTP forwarding',curl('http://127.0.0.1:18080/').stdout=='linux-audit-origin-ok')
    check('CONNECT tunnel',curl('http://127.0.0.1:18080/','--proxytunnel').stdout=='linux-audit-origin-ok')
    pac=run('curl','-fsS','--max-time','5','http://127.0.0.1:17894/proxy.pac').stdout
    check('PAC download','FindProxyForURL' in pac and 'PROXY 127.0.0.1:17894' in pac)
    rules([{'type':'ip-cidr','value':'127.0.0.1/32','action':'reject'}])
    check('routing hot reload rejects request',curl('http://127.0.0.1:18080/','-o','/dev/null','-w','%{http_code}').stdout=='502')
    check('routing list',json.loads(cli('routing','list','--json').stdout)[0]['action']=='reject')
    rules([])
    cli('http-proxy','set',input=json.dumps({'enabled':True,'port':17894,'auth':'basic','username':'audit','password':'audit-test-only'}))
    check('HTTP auth rejects missing credentials',curl('http://127.0.0.1:18080/','-o','/dev/null','-w','%{http_code}').stdout=='407')
    check('HTTP auth accepts credentials',curl('http://127.0.0.1:18080/','--proxy-user','audit:audit-test-only').stdout=='linux-audit-origin-ok')
    cli('http-proxy','set',input=json.dumps({'enabled':True,'port':17894,'auth':'none','username':''}))
    for kind,port in [('http',18081),('socks5',18082)]:
        cli('egress','proxy','--type',kind,'--host','127.0.0.1','--port',str(port))
        rules([{'type':'domain','value':'audit.invalid','action':'proxy'}])
        count=len(upstream)
        response=curl('http://audit.invalid:18080/').stdout
        check(kind+' upstream proxy + domain rule',response=='linux-audit-origin-ok' and any(k==kind and 'audit.invalid:18080' in dest for k,dest in upstream[count:]))
    fake=run('dig','+short','+time=2','+tries=1','-b',status['gateway']['LocalIP'],'@'+status['gateway']['LocalIP'],'-p','1053','audit.invalid','A').stdout.strip()
    check('fake-IP DNS',fake.startswith('198.18.'))
    cli('learning','ignore','audit.invalid')
    check('learning ignore','audit.invalid' in api('stats')['fallback']['ignored'])
    cli('learning','restore','audit.invalid')
    check('learning restore','audit.invalid' not in api('stats')['fallback']['ignored'])
    snapshot=json.loads(cli('agent','snapshot').stdout)
    check('agent runtime snapshot','runtime' in snapshot)
    check('API devices endpoint','subnet' in api('devices'))
    check('hotspot reports unsupported',not json.loads(cli('hotspot','status').stdout).get('available',False))
    check('system-proxy status falls back to LAN egress','LAN 出口:' in cli('system-proxy','status').stdout)
    cli('skill','export','--output','/tmp/audit-skill.zip')
    check('skill export',pathlib.Path('/tmp/audit-skill.zip').is_file())
    cli('restart')
    check('restart keeps configuration',api('stats')['egress']=='proxy' and len(json.loads(cli('routing','list','--json').stdout))==1)
    cli('egress','direct')
    check('switch back to direct',api('stats')['egress']=='direct')
    cli('stop')
    check('stop removes iptables','lan-proxy-gateway' not in run('iptables-save').stdout)
    check('stop restores forwarding',pathlib.Path('/proc/sys/net/ipv4/ip_forward').read_text().strip()=='0')
    check('stopped status',not json.loads(cli('status','--json').stdout)['running'])
    check('no daemon panic', 'panic' not in pathlib.Path('/root/.config/lan-proxy-gateway/gateway.log').read_text())
    print('RESULT',len(checks),'checks passed',flush=True)
except Exception:
    log=pathlib.Path('/root/.config/lan-proxy-gateway/gateway.log')
    if log.exists(): print('DAEMON LOG',log.read_text()[-8000:],flush=True)
    raise
finally:
    cli('stop',ok=False)
