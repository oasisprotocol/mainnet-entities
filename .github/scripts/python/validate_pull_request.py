#!/usr/bin/env python3
import os
import sys
from github import Github


class InvalidEntityPR(Exception):
    pass


def validate_entity_pull_request(gh, repo, pr_number):
    """Validate if this pull request is a valid entity pull request."""
    repo = gh.get_repo(repo)
    pr = repo.get_pull(pr_number)
    pr_creator = pr.user.login

    is_valid = False

    expected_filename = "entities/%s-entity.tar.gz" % pr_creator

    changed_files = list(pr.get_files())
    if len(changed_files) > 1:
        raise InvalidEntityPR(
            'This PR contains more than an entity change. That is not allowed without explicit review.'
        )

    if changed_files[0].filename == expected_filename:
        is_valid = True

    if not is_valid:
        raise InvalidEntityPR(
            'The entity file is expected to be named %s. Please remediate.' % expected_filename
        )


def main():
    token = os.environ.get('GITHUB_TOKEN')
    github_ref = os.environ.get('GITHUB_REF', '')
    if not token:
        print('No github token specified')
        sys.exit(1)
    if not github_ref:
        print('No github_ref specified')
        sys.exit(1)

    try:
        pr_number = int(github_ref.split('/')[2])
    except TypeError:
        print("This might not be a PR or something is wrong")
        sys.exit(1)
    gh = Github(token)

    print("Validating PR #%d" % pr_number)

    try:
        validate_entity_pull_request(
            gh,
            'oasisprotocol/amber-network-entities',
            pr_number
        )
    except InvalidEntityPR as e:
        print(e)
        sys.exit(1)


if __name__ == '__main__':
    main()
