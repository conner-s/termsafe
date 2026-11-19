#!/bin/sh
# Fetches Unicode's lists of variation sequences and runs the test that holds
# termsafe's rule to every one of them: TestSelectorsAgainstUnicodeData.
#
# For a maintainer, by hand. The files are Unicode's, under the Unicode
# License, and none of them is kept in this repository; the ordinary tests
# check a table of the kinds of sequence instead (TestSelectorClasses). Run
# this after changing takesSelector, and when a Go release brings a new
# Unicode version: with the new version's number, under that Go release.
#
#   scripts/unicode-data.sh [unicode version] [directory]
#
# The version defaults to 17.0.0, the directory to one under $TMPDIR, which
# is kept so that a second run does not download again. IVD_DATE picks the
# release of the Ideographic Variation Database, 2025-07-14 if not set.
set -eu

version=${1:-17.0.0}
dir=${2:-${TMPDIR:-/tmp}/termsafe-unicode-$version}
ivd=${IVD_DATE:-2025-07-14}

mkdir -p "$dir"
fetch() {
	if [ ! -s "$dir/$1" ]; then
		echo "fetching $2"
		curl -fsS -o "$dir/$1" "$2"
	fi
}
fetch StandardizedVariants.txt "https://www.unicode.org/Public/$version/ucd/StandardizedVariants.txt"
fetch emoji-variation-sequences.txt "https://www.unicode.org/Public/$version/ucd/emoji/emoji-variation-sequences.txt"
fetch IVD_Sequences.txt "https://www.unicode.org/ivd/data/$ivd/IVD_Sequences.txt"

cd "$(dirname "$0")/.."
TERMSAFE_UNICODE_DATA=$dir TERMSAFE_UNICODE_VERSION=$version \
	go test -count=1 -run '^(TestSelectorsAgainstUnicodeData|TestUnicodeVersion)$' -v .
