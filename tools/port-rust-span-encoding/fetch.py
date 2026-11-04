#!/usr/bin/env python3
"""Explicit cold-cache dependency fetch; behavioral generators remain offline."""
import pathlib
import sys

import prepare


def main():
    env = prepare.environment()
    destination = prepare.ROOT / '.toolchain/oracles/span-encoding-reference'
    prepare.prepare(destination, env)
    cargo = prepare.selected('FERRUGO_CARGO', pathlib.Path('.toolchain/rust/bin/cargo'), 'cargo')
    prepare.execute([cargo, 'fetch', '--locked'], env, destination)
    prepare.execute([sys.executable, str(prepare.HERE / 'prepare.py'), '--offline'], env)
    print('Fetched and verified locked span-reference dependencies.')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(error, file=sys.stderr)
        sys.exit(1)
