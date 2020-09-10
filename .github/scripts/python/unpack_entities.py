#!/usr/bin/env python3
import json
import os
import sys
import tempfile
import tarfile
import logging
import base64
import cbor
import subprocess

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


def address(public_key):
    return subprocess.check_output([
        '/tmp/oasis-node',
        'stake',
        'pubkey2address',
        '--public_key', public_key
    ])


class EntityPackage(object):
    @classmethod
    def load(cls, entity_owner, unpacked_entity_dir_path):
        package = cls(entity_owner, unpacked_entity_dir_path)

        if not package.validate_expected_files():
            return package

        package.load_entity_descriptor()
        package.load_node_descriptor()
        return package

    def __init__(self, owner, path):
        self._path = path
        self._owner = owner
        self._entity_descriptor = None
        self._node_descriptor = None

    def validate_expected_files(self):
        # Validate that the expected directory structure exists
        for expected_file_name in EXPECTED_FILES:
            expected_file_path = os.path.join(self._path, expected_file_name)

            if not os.path.isfile(expected_file_path):
                logger.warning('Expected file "%s" missing' %
                               expected_file_path)
                return False
        return True

    def is_valid(self):
        if self._entity_descriptor is None:
            return False
        if self._node_descriptor is None:
            return False
        return True

    def load_entity_descriptor(self):
        logger.info("loading entity descriptor for %s" % self._owner)
        # Ensure that the node is properly loaded into the
        # FIXME we should do this check using something written with oasis-core as a
        # library. This is quick and dirty.
        entity_genesis_path = os.path.join(
            self._path, 'entity/entity_genesis.json')

        with open(entity_genesis_path) as entity_genesis_file:
            entity_genesis = json.load(entity_genesis_file)

        entity_descriptor = cbor.loads(base64.b64decode(
            entity_genesis['untrusted_raw_value']))

        if 'v' not in entity_descriptor:
            logger.warning(
                'Expected entity descriptor at "%s" to have a version. Please update your entity.' % entity_genesis_path)
            return

        self._entity_descriptor = entity_descriptor

    def load_node_descriptor(self):
        if self._entity_descriptor is None:
            raise Exception(
                "cannot load node descriptor. object state invalid")
        logger.info("loading node descriptor for %s" % self._owner)

        node_genesis_path = os.path.join(
            self._path, 'node/node_genesis.json')

        with open(node_genesis_path) as node_genesis_file:
            node_genesis = json.load(node_genesis_file)

        node_descriptor = cbor.loads(
            base64.b64decode(node_genesis['untrusted_raw_value']))

        entity_nodes = self._entity_descriptor['nodes'] or []

        if not node_descriptor['id'] in entity_nodes:
            logger.warning('Expected node to be added to entity')
            return

        self._node_descriptor = node_descriptor

    @property
    def address(self):
        if self._entity_descriptor is None:
            return "unknown_entity_package_invalid"
        return address(base64.b64encode(self._entity_descriptor["id"])).decode('utf-8').strip()

    @property
    def entity_id(self):
        if self._entity_descriptor is None:
            return "unknown_entity_package_invalid"
        return base64.b64encode(self._entity_descriptor["id"]).decode('utf-8').strip()

    @property
    def name(self):
        return self._owner

    @property
    def node_id(self):
        if self._node_descriptor is None:
            return "unknown_entity_package_invalid"
        return base64.b64encode(self._node_descriptor["id"]).decode('utf-8').strip()


def unpack_entities(src_entities_dir_path, dest_entities_dir_path):
    # HACK TO TRACK ALL NODE/ENTITY IDs
    with open('/tmp/entity_list.csv', 'w') as entity_list_file:
        entity_list_file.write('name,address,entity_id,node_id\n')

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
                logger.info('Unpack package for entity owner "%s"' %
                            entity_owner)
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

                package = EntityPackage.load(
                    entity_owner, unpacked_entity_dir_path)

                if not package.is_valid():
                    invalid_entity_packages.append(entity_owner)
                else:
                    logger.info('Entity owned by "%s" is valid' % entity_owner)

                entity_list_file.write('%s,%s,%s,%s\n' % (
                    package.name,
                    package.address,
                    package.entity_id,
                    package.node_id,
                ))

        if len(invalid_entity_packages) > 0:
            for entity_owner in invalid_entity_packages:
                logger.error('Invalid Entity for %s' % entity_owner)
            raise InvalidEntitiesDetected()


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
