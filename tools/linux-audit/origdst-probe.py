import json,socket,struct
from harness import require_container
require_container()
sock=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
sock.setsockopt(socket.SOL_IP,20,1)
sock.bind(('0.0.0.0',18883))
sock.settimeout(35)
open('/tmp/origdst-ready','w').write('ready')
msg,controls,flags,client=sock.recvmsg(4096,256)
out={'client':client,'payload':msg.decode(),'original_destination':[]}
for level,kind,data in controls:
    if level==socket.SOL_IP and kind==20:
        out['original_destination'].append({'ip':socket.inet_ntoa(data[4:8]),'port':struct.unpack('!H',data[2:4])[0]})
open('/tmp/origdst-result.json','w').write(json.dumps(out))
