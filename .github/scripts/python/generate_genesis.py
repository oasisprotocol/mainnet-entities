import os
import sys
import subprocess
import json
from datetime import datetime


def add_entities_from_directory(genesis_command, entity_dir, add_nodes):
    for entity_name in os.listdir(entity_dir):
        if os.path.isfile(os.path.join(entity_dir, entity_name)):
            continue
        genesis_command.extend([
            '--entity', os.path.join(entity_dir,
                                     entity_name, 'entity/entity_genesis.json'),
        ])

        if add_nodes:
            genesis_command.extend([
                '--node', os.path.join(entity_dir,
                                       entity_name, 'node/node_genesis.json'),
            ])


def main():
    # Find all of the entity_genesis.json files and node_genesis.json files
    unpacked_entities_path = os.path.abspath(sys.argv[1])
    test_entities_path = ""
    try:
        test_entities_path = os.path.abspath(sys.argv[2])
    except IndexError:
        pass

    # Hacky overrides for running locally.
    output_path = os.environ.get('GENESIS_OUTPUT_PATH', '/tmp/genesis.json')
    staking_path = os.environ.get('STAKING_GENESIS_PATH', '/tmp/staking.json')
    oasis_node_path = os.environ.get('OASIS_NODE_PATH', '/tmp/oasis-node')
    oasis_test_only = os.environ.get('OASIS_TEST_ONLY', 'false') == 'true'

    timestamp = datetime.now().strftime('%Y-%m-%d-%s')
    chain_id = 'quest-%s' % timestamp
    if oasis_test_only:
        chain_id = 'test-%s' % timestamp

    genesis_command = [
        oasis_node_path, 'genesis', 'init',
        '--genesis.file', output_path,
        '--chain.id', chain_id,
        '--staking', staking_path,
        '--epochtime.tendermint.interval', '600',
        '--consensus.tendermint.timeout_commit', '5s',
        '--consensus.tendermint.empty_block_interval', '0s',
        '--consensus.tendermint.max_tx_size', '32kb',
        '--consensus.tendermint.max_evidence_age_blocks', '100000',
        '--consensus.tendermint.max_evidence_age_time', '172800000000000ns',
        '--consensus.backend', 'tendermint',
        '--scheduler.max_validators', '100',
        '--scheduler.max_validators_per_entity', '1',
    ]

    add_entities_from_directory(
        genesis_command, unpacked_entities_path, add_nodes=not oasis_test_only)

    if oasis_test_only:
        add_entities_from_directory(
            genesis_command, test_entities_path, add_nodes=oasis_test_only)
        genesis_command.extend(['--scheduler.min_validators', '3'])
    else:
        genesis_command.extend(['--scheduler.min_validators', '10'])

    # Run genesis command
    subprocess.check_call(genesis_command)

    print("Pretty printing genesis")
    # Pretty print genesis json
    genesis = json.load(open(output_path, 'r'))
    json.dump(genesis, open(output_path, 'w'), indent=2, sort_keys=True)


if __name__ == '__main__':
    main()
