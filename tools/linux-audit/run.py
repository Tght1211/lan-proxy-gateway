#!/usr/bin/env python3
"""Run the Linux CLI audit against an existing executable in disposable Docker containers."""

import argparse
import json
from pathlib import Path
import subprocess
import sys
import time
import uuid


def command(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path, help='Linux executable matching the Docker engine architecture')
    parser.add_argument('suite', choices=['smoke', 'udp-transition', 'transparent'])
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    if not binary.is_file():
        parser.error('binary must be a file')
    image = 'lan-proxy-gateway-linux-audit:local'
    command('docker', 'build', '-t', image, str(Path(__file__).resolve().parent))
    identifier = 'lpg-audit-' + uuid.uuid4().hex[:12]
    network = identifier + '-net'
    names = []
    network_created = False
    try:
        command('docker', 'network', 'create', network, stdout=subprocess.DEVNULL)
        network_created = True
        common = ['docker', 'run', '--init', '--network', network, '--cap-add', 'NET_ADMIN',
                  '--security-opt', 'systempaths=unconfined',
                  '--mount', f'type=bind,src={binary},dst=/gateway,readonly']
        server = identifier + '-server'
        names.append(server)
        if args.suite != 'transparent':
            return subprocess.run([*common, '--rm', '--name', server, image,
                                   'python3', '/audit/' + args.suite + '.py', '/gateway']).returncode

        command(*common, '-d', '--name', server, image,
                'python3', '/audit/transparent-server.py', '/gateway', stdout=subprocess.DEVNULL)
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            log = command('docker', 'logs', server, capture_output=True, text=True).stdout
            if 'READY' in log.splitlines():
                break
            state = command('docker', 'inspect', '--format', '{{.State.Running}}', server,
                            capture_output=True, text=True).stdout.strip()
            if state != 'true':
                command('docker', 'logs', server)
                raise RuntimeError('Gateway test container exited before readiness')
            time.sleep(0.2)
        else:
            raise TimeoutError('Gateway test container did not become ready within 30 seconds')
        info = json.loads(command('docker', 'inspect', server, capture_output=True, text=True).stdout)[0]
        gateway_ip = info['NetworkSettings']['Networks'][network]['IPAddress']
        client = identifier + '-client'
        names.append(client)
        result = subprocess.run(['docker', 'run', '--rm', '--init', '--name', client,
                                 '--network', network, '--cap-add', 'NET_ADMIN', image,
                                 'python3', '/audit/transparent-client.py', gateway_ip])
        if result.returncode:
            subprocess.run(['docker', 'exec', server, 'cat',
                            '/root/.config/lan-proxy-gateway/gateway.log'], check=False)
        return result.returncode
    finally:
        # Only remove resources created for this invocation, including on Ctrl-C.
        for name in reversed(names):
            subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, check=False)
        if network_created:
            subprocess.run(['docker', 'network', 'rm', network], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, check=False)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except (OSError, RuntimeError, subprocess.CalledProcessError) as exc:
        print(f'Audit failed: {exc}', file=sys.stderr)
        sys.exit(1)
