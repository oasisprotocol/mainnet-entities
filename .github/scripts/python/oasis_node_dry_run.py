"""
Hacky script to do a quick dry run of the oasis-node against the current genesis.

Usage:

    oasis_node_dry_run.py [oasis_node_path] [genesis_path] [seed]
"""
import subprocess
import os
import sys
import tempfile
import shutil

NODE_CONFIG_YML = """
datadir: %(tmp_dir)s/node
genesis:
  file: %(tmp_dir)s/node/genesis/genesis.json

tendermint:
  abci:
    prune:
      strategy: none
  core:
    listen_address: tcp://0.0.0.0:26656

  db:
    backend: badger
  debug:
    addr_book_lenient: false
"""


def main():
    oasis_node_path = sys.argv[1]
    genesis_path = sys.argv[2]

    with tempfile.TemporaryDirectory() as tmp_dir:
        print('Creating storage directory %s at' % tmp_dir)

        # Initialize directory
        os.mkdir(os.path.join(tmp_dir, 'node'), 0o700)
        os.mkdir(os.path.join(tmp_dir, 'node/genesis'), 0o700)

        with open(os.path.join(tmp_dir, 'node/config.yml'), 'w') as config_file:
            config_file.write(
                NODE_CONFIG_YML % dict(
                    tmp_dir=tmp_dir
                )
            )

        # Copy the genesis file
        shutil.copyfile(genesis_path, os.path.join(
            tmp_dir, 'node/genesis/genesis.json'))

        command = [
            oasis_node_path, '--config',
            os.path.join(tmp_dir, 'node/config.yml')
        ]

        process = subprocess.Popen(command)

        # Run the node
        try:
            process.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            process.terminate()
        if process.wait() != 0:
            print('Error executing the process')
            sys.exit(1)


if __name__ == "__main__":
    main()
