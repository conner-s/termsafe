#!/bin/sh
# Fails when a tracked file names a host or address that only exists on a
# private network. This repository is mirrored to a public one, and every
# release becomes a module zip the Go proxy keeps for good, so a private
# hostname written here is published and cannot be taken back.
#
# The patterns are generic on purpose: a list of the owner's own hostnames
# would publish them from this file.
#
#   - an IPv4 address in a private range (10/8, 172.16/12, 192.168/16)
#   - a hostname with a label that marks it as private: .internal, .lan,
#     .home.arpa
#
# Write "the owner's private forge", an address from a documentation range
# (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24) or example.com instead.
#
# Usage: scripts/host-neutral.sh   (from the repository root; needs git)
set -eu

private_ip='(^|[^0-9.])(10\.[0-9]{1,3}|192\.168|172\.(1[6-9]|2[0-9]|3[01]))\.[0-9]{1,3}\.[0-9]{1,3}([^0-9.]|$)'
private_host='[a-z0-9-]+\.(internal|lan|home\.arpa)(\.[a-z0-9-]+)*([^a-z0-9.-]|$)'

# A check that lists nothing finds nothing: outside a work tree, or where git
# refuses the directory, say so instead of passing.
if [ "$(git ls-files | wc -l)" -eq 0 ]; then
  echo "host-neutral: git lists no tracked files here; run it from the repository root" >&2
  exit 1
fi

# -I skips binary files; the script's own patterns are not hostnames.
found=$(git ls-files -z -- . ':!scripts/host-neutral.sh' \
  | xargs -0 grep -InE -e "$private_ip" -e "$private_host" -- 2>/dev/null || true)

if [ -n "$found" ]; then
  echo "host-neutral: these lines name a private host or address:" >&2
  echo "$found" >&2
  exit 1
fi
echo "host-neutral: no tracked file names a private host or address"
