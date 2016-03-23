#!/usr/bin/env python
#
# From: https://github.com/helix-collective/post-commit-pr
#
# Helper to follow the workflow of having one git commit per code review/PR. This script
# automatically pushes up the relevant branches to create a github PR (if they don't exist)
# or updates the given branches if a PR already exists.
#
# It also ammends the commit to include a link to the PR, for future reference.
#
# USAGE: tools/pr.py 
#    pushes up whatever commit is current at HEAD as a github pull request, if it's already part of a PR
#    then it simply updates the PR id specified in the commit message
#
#    If you would like to push up PRs for changes other than the HEAD commit, then perform an interactive rebase
#    and push up PRs as you go along.

import subprocess
import os
import sys
import getpass
import httplib
import json
import random
from base64 import encodestring
import re

TOKEN_FILE = os.path.expanduser("~/.postcommit-github-access-token")
USER = os.environ['USER']

def call(*cmd):
    return subprocess.check_output(cmd)

def show(fmt, rev):
    return call("git", "show", "-s", "--format=" + fmt, rev).strip()

def repo():
    remote = call("git", "config", "--get", "remote.origin.url")
    return remote.split(':')[-1].replace('.git', '').strip()

def get_branch(rev):
    current_ref=show('%H', rev)
    for l in call("git", "for-each-ref", 'refs/remotes/origin/' + USER + "/PR").splitlines():
        if 'HEAD' in l:
            continue

        if current_ref in l:
            return l.split()[2].replace('refs/remotes/origin/', '').strip()


def get_pr_id(body):
    for l in body.splitlines():
        if 'PR:' in l:
            return l.split('/')[-1].strip()

def branch_name_for(rev):
    name = show('%s', rev).lower()

    # replace spaces
    name = name.replace(' ', '-').replace('\t', '-').lower()

    # filter characters that are valid in branches
    return re.sub('[^A-Za-z-_]', '', name)

class Github(object):
    def __init__(self):
        self.conn = httplib.HTTPSConnection('api.github.com', 443)
        self.conn.connect()
        self.headers = {
            "User-Agent": "httplib/python",
            "Content-Type": "application/json"
        }

    def set_basic_auth(self, username, password):
        self.headers['Authorization'] = 'Basic %s' % encodestring("%s:%s" % (username, password)).replace('\n', '')

    def set_auth_token(self, token):
        self.headers['Authorization'] = 'token ' + token

    def post(self, path, data):
        self.conn.request('POST', path, json.dumps(data), self.headers)
        return self._err_or_val(self.conn.getresponse())

    def get(self, path):
        self.conn.request('GET', path, headers=self.headers)
        return self._err_or_val(self.conn.getresponse())

    def _err_or_val(self, resp):
        resp_body = resp.read()
        retval = json.loads(resp_body)

        if resp.status < 200 or resp.status >= 300:
            raise Exception("Github API Error", retval)

        return retval


github = Github()

try:
    github.set_auth_token(open(TOKEN_FILE).read().strip())
except:
    sys.stdout.write("Github username: ")
    username = sys.stdin.readline().strip()
    password = getpass.getpass()
    github.set_basic_auth(username, password)
    resp = github.post("/authorizations", {
        "scopes": ["repo", "public_repo"],
        "note": "post-commit-pr-" + str(random.randint(0, 10000))
    })

    with open(TOKEN_FILE, 'w') as f:
        f.write(resp['token'].strip())

    github.set_auth_token(resp['token'].strip())

subject = show('%s', 'HEAD')
body = show('%b', 'HEAD')
pr_id = get_pr_id(body)

if pr_id:
    # get the PR's base and head
    result = github.get('/repos/' + repo() + '/pulls/' + pr_id)
    head = result['head']['ref']
    base = result['base']['ref']

    # update those branches
    call("git", "push", "-f", "origin", "HEAD~1:refs/heads/%s" % base)
    call("git", "push", "-f", "origin", "HEAD:refs/heads/%s" % head)

    # That's all we need to do for a PR update
    print "PR Updated: ", result['html_url']
else:
    # create PR branch for BASE (if one doesn't already exist)
    base = get_branch('HEAD~1')
    if not base:
        base = USER + '/PR/' + branch_name_for('HEAD~1')
        call("git", "push", "-f", "origin", "HEAD~1:refs/heads/%s" % base)

    # create PR branch for HEAD (if one doesn't already exist)
    head = get_branch('HEAD')
    if not head:
        head = USER + '/PR/' + branch_name_for('HEAD')
        call("git", "push", "-f", "origin", "HEAD:refs/heads/%s" % head)

    # create a new PR
    result = github.post('/repos/' + repo() + '/pulls', {
        "title": subject,
        "head": head,
        "base": base,
        "body": body,
    })

    # ammend commit, with a link to the PR
    call("git", "commit", "--amend", "-m", "%s\n\n%s\n\nPR: %s" %
            (subject, body, result['html_url']))
    print "New PR Created:", result['html_url']
