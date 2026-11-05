import json
import pathlib
import socket
import sys
import threading
import time

from harness import api, check, checks, cli, curl, initialize, rules, run, upstream

initialize(sys.argv[1])

try:
    cli('init')
    cli('start')
    cli('egress','proxy','--type','socks5','--host','127.0.0.1','--port','18082')
    before=api('stats').get('udp_relay')
    print('UDP_AFTER_DIRECT_TO_PROXY',json.dumps(before),flush=True)
    print('UDP_FIREWALL_RULE_PRESENT', '17893' in run('iptables-save').stdout,flush=True)
    cli('restart')
    after=api('stats').get('udp_relay')
    print('UDP_AFTER_RESTART',json.dumps(after),flush=True)
    check('UDP listener exists immediately after direct-to-proxy switch', before is not None)
finally:
    cli('stop',ok=False)
