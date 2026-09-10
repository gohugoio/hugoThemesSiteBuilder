#!/usr/bin/env bash
# Like check.sh, but keeps the check's work dir, starts a Hugo server in the
# demo site for manual inspection, and removes the work dir(s) again when
# the server exits (Ctrl-C).
#
# Usage: ./checko.sh <module-path>|<PR-number>
#
# With multiple themes, the server runs in the last one's demo site, but
# all work dirs are cleaned up.

out=$(mktemp)
workdirs=""

cleanup() {
	popd >/dev/null 2>&1
	local d
	for d in $workdirs; do
		if [[ -d "$d" && "$d" == *theme-check-* ]]; then
			rm -rf "$d" && echo "checko: removed $d"
		fi
	done
	rm -f "$out"
}
trap cleanup EXIT

# The check may exit non-zero (that is often why we are inspecting); keep
# going as long as there is a work dir to serve.
go run main.go check -keep "$@" 2>&1 | tee "$out"

workdirs=$(sed -n 's/^keeping work dir //p' "$out")
workdir=$(echo "$workdirs" | tail -1)
if [[ -z "$workdir" ]]; then
	echo "checko: no work dir found in the check output" >&2
	exit 1
fi
if [[ ! -d "$workdir/demosite" ]]; then
	echo "checko: no demo site in $workdir (module resolution failed?)" >&2
	exit 1
fi

pushd "$workdir/demosite" >/dev/null || exit 1
"${HUGOTHEMES_HUGO_LATEST:-hugo}" server -FONDM
