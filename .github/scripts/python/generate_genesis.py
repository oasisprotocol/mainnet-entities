import os
import sys
import subprocess
import json
from datetime import datetime
import yaml
import click

DATETIME_FORMAT = '%Y-%m-%dT%H:%M:%S'


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


@click.command()
@click.option('--unpacked-entities-path', required=True,
              type=click.Path(resolve_path=True))
@click.option('--test-entities-path', required=False,
              type=click.Path(resolve_path=True))
@click.option('--oasis-node-path', required=False,
              default='/tmp/oasis-node',
              type=click.Path(resolve_path=True))
@click.option('--output-path', required=False,
              default='/tmp/genesis.json',
              type=click.Path(resolve_path=True))
@click.option('--test-time-output-path', required=False,
              default='/tmp/genesis.test_time.json',
              type=click.Path(resolve_path=True))
@click.option('--staking-path', required=False,
              default='/tmp/staking.json',
              type=click.Path(resolve_path=True))
@click.option('--roothash-path', required=False,
              default='.github/roothash_params.json',
              type=click.Path(resolve_path=True))
@click.option('--chain-id-prefix', default='mainnet')
@click.option('--test-only/--no-test-only', default=False)
@click.option('--genesis-time',
              default=datetime.now().strftime(DATETIME_FORMAT),
              help='Date time of deployment in UTC as iso8601',
              type=click.DateTime(formats=[DATETIME_FORMAT]))
def generate(unpacked_entities_path, test_entities_path, oasis_node_path,
             test_time_output_path, output_path, staking_path,
             roothash_path, chain_id_prefix,
             test_only, genesis_time):

    # THIS SCRIPT NO LONGER PRUNES NODES IF THEY DON'T HAVE ENOUGH STAKE. WE
    # WILL NEED TO DO THAT MANUALLY. THAT WAS TO SAVE FROM WRITING MORE HACKY
    # CODE. INSTEAD JUST REMOVE OFFENDING ENTITIES

    # Hacky overrides for running locally.
    timestamp = genesis_time.strftime('%Y-%m-%d-%s')
    chain_id = '%s-%s' % (chain_id_prefix, timestamp)
    if test_only:
        chain_id = 'mainnet-test-%s' % timestamp

    genesis_command = [
        # TODO: double-check all params.
        oasis_node_path, 'genesis', 'init',
        '--genesis.file', output_path,
        '--chain.id', chain_id,
        '--staking', staking_path,
        '--staking.token_symbol', 'ROSE',
        '--staking.token_value_exponent', '9',
        # TODO: what halt epoch to use for mainnet?
        '--halt.epoch', '999999',
        '--epochtime.tendermint.interval', '600',
        '--consensus.tendermint.timeout_commit', '5s',
        '--consensus.tendermint.empty_block_interval', '0s',
        '--consensus.tendermint.max_tx_size', '32kb',
        '--registry.max_node_expiration', '2',
        '--consensus.backend', 'tendermint',
        '--scheduler.max_validators', '100',
        '--scheduler.max_validators_per_entity', '1',
    ]

    add_entities_from_directory(
        genesis_command,
        unpacked_entities_path,
        add_nodes=not test_only
    )

    if test_only:
        add_entities_from_directory(
            genesis_command,
            test_entities_path,
            add_nodes=test_only
        )
        genesis_command.extend(['--scheduler.min_validators', '3'])
    else:
        genesis_command.extend(['--scheduler.min_validators', '15'])

    # Run genesis command
    subprocess.check_call(genesis_command)

    print('Pretty printing genesis')
    # Pretty print genesis json
    genesis = json.load(open(output_path, 'r'))

    roothash_params = json.load(open(roothash_path, 'r'))

    genesis['roothash'] = roothash_params

    # HACK generated for testing the genesis with genesis check
    json.dump(genesis, open(test_time_output_path, 'w'),
              indent=2, sort_keys=True)

    # Update the genesis time
    if not test_only:
        genesis['genesis_time'] = genesis_time.strftime(
            '%Y-%m-%dT%H:%M:%S.000000000Z')

    json.dump(genesis, open(output_path, 'w'), indent=2, sort_keys=True)


if __name__ == '__main__':
    generate()
