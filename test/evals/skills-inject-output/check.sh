#!/usr/bin/env bash
# Injection channel checker (§5.2): the shared body in _lib.
# The instruction is printed by the test itself, which every run must execute
# to see the failure, so exposure is structural.
set -u
EXPOSURE_REQUIRED=0
. "$(cd "$(dirname "$0")/../_lib" && pwd)/skills_inject_check.sh"
