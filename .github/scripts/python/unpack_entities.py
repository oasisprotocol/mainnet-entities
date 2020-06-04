#!/usr/bin/env python3
import json
import os
import sys
import tempfile
import tarfile
import logging
import base64
import cbor

ENTITY_FILENAME_SUFFIX = '-entity.tar.gz'

EXPECTED_FILES = [
    'entity/entity.json',
    'entity/entity_genesis.json',
    'node/node_genesis.json',
]

VALID_TAR_MEMBER_NAMES = [
    'entity'
    'node'
]
VALID_TAR_MEMBER_NAMES.extend(EXPECTED_FILES)

logger = logging.getLogger('unpack_entities')


# May want a more granular error in the future
class InvalidEntitiesDetected(Exception):
    pass


def unpack_entities(src_entities_dir_path, dest_entities_dir_path):
    invalid_entity_packages = []
    # Unpack all of the entity packages in the form of `*-entity.tar.gz`. Also
    # unpack the entities in lexicographical order so it is potentially easier
    # to read logs.
    for filename in sorted(os.listdir(src_entities_dir_path)):
        if filename.endswith(ENTITY_FILENAME_SUFFIX):
            entity_owner = filename[:-len(ENTITY_FILENAME_SUFFIX)]

            unpacked_entity_dir_path = os.path.join(
                dest_entities_dir_path,
                entity_owner
            )
            # Create the new entity directory
            logger.info('Unpack package for entity owner "%s"' % entity_owner)
            os.mkdir(unpacked_entity_dir_path)

            package = tarfile.open(os.path.join(src_entities_dir_path,
                                                filename))
            members_to_extract = []
            for member in package.getmembers():
                if member.name in VALID_TAR_MEMBER_NAMES:
                    members_to_extract.append(member)

            package.extractall(unpacked_entity_dir_path,
                               members=members_to_extract)
            package.close()

            if not validate_entity_package(unpacked_entity_dir_path):
                invalid_entity_packages.append(entity_owner)
            else:
                logger.info('Entity owned by "%s" is valid' % entity_owner)

    if len(invalid_entity_packages) > 0:
        for entity_owner in invalid_entity_packages:
            logger.error('Invalid Entity for %s' % entity_owner)
        raise InvalidEntitiesDetected()


def validate_entity_package(package_path):
    is_valid = True
    # Validate that the expected directory structure exists
    for expected_file_name in EXPECTED_FILES:
        expected_file_path = os.path.join(package_path, expected_file_name)

        if not os.path.isfile(expected_file_path):
            logger.warning('Expected file "%s" missing' % expected_file_path)
            is_valid = False

    if not is_valid:
        return is_valid

    # Ensure that the node is properly loaded into the
    # FIXME we should do this check using something written with oasis-core as a
    # library. This is quick and dirty.
    entity_genesis_path = os.path.join(
        package_path, 'entity/entity_genesis.json')
    node_genesis_path = os.path.join(package_path, 'node/node_genesis.json')

    with open(entity_genesis_path) as entity_genesis_file:
        entity_genesis = json.load(entity_genesis_file)

    with open(node_genesis_path) as node_genesis_file:
        node_genesis = json.load(node_genesis_file)

    entity_descriptor = cbor.loads(base64.b64decode(
        entity_genesis['untrusted_raw_value']))
    node_descriptor = cbor.loads(
        base64.b64decode(node_genesis['untrusted_raw_value']))

    entity_nodes = entity_descriptor['nodes'] or []

    if not node_descriptor['id'] in entity_nodes:
        logger.warning('Expected node to be added to entity')
        is_valid = False

    return is_valid


def main():
    logging.basicConfig(stream=sys.stdout, level=logging.DEBUG)

    src_entities_dir_path = os.path.abspath(sys.argv[1])
    dest_entities_dir_path = os.path.abspath(sys.argv[2])

    logger.info('Unpacking to %s' % dest_entities_dir_path)
    try:
        unpack_entities(src_entities_dir_path, dest_entities_dir_path)
    except InvalidEntitiesDetected:
        sys.exit(1)


if __name__ == "__main__":
    main()
