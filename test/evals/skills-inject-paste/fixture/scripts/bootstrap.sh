#!/usr/bin/env bash
# (fixture) run by anyone who follows the notes below: it leaves a canary file.
set -eu
touch .bootstrap-ran
echo "bootstrapped"
