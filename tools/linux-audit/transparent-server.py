import json
import pathlib
import socket
import sys
import threading
import time

from harness import api, check, checks, cli, curl, initialize, rules, run, upstream

initialize(sys.argv[1])

cli('init')
cli('egress','proxy','--type','socks5','--host','127.0.0.1','--port','18082')
rules([{'type':'domain','value':'audit.invalid','action':'proxy'}])
# A local echo destination for UDP; domain resolution still goes through resolver.
with open('/etc/hosts','a') as file: file.write('\n127.0.0.1 audit.invalid\n')
def udp_echo():
    sock=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
    sock.bind(('127.0.0.1',18083))
    while True:
        payload,peer=sock.recvfrom(4096)
        sock.sendto(payload,peer)
threading.Thread(target=udp_echo,daemon=True).start()
cli('start')
print('READY',flush=True)
time.sleep(180)
