#!/usr/bin/env bash
# The ordinary shell route for changeset-pins: discover the rollout from the
# files, derive each tag, rewrite both files, then verify. One programme.
set -euo pipefail
cd "${1:-.}"

salt=$(python3 -c 'import json;print(json.load(open("releases.json"))["salt"])')
stage=$(python3 -c 'import json;print(json.load(open("releases.json"))["rollout"]["stage"])')
mapfile -t names < <(python3 - "$stage" <<'PY'
import json,sys
d=json.load(open("releases.json"))
print("\n".join(d["stages"][sys.argv[1]]))
PY
)

for name in "${names[@]}"; do
  f="services/$name/service.yaml"
  [ -f "$f" ] || continue
  replicas=$(awk '/^replicas:/{print $2}' "$f")
  [ "${replicas:-0}" -ge 2 ] || continue
  version=$(python3 -c 'import json,sys;print(json.load(open("releases.json"))["services"][sys.argv[1]]["version"])' "$name")
  tag="$version-$(printf '%s:%s:%s' "$salt" "$name" "$replicas" | sha256sum | cut -c1-7)"
  rid=$(printf '%s:%s' "$name" "$tag" | sha256sum | cut -c1-10)
  sed -i "s|^image: ghcr.io/acme/$name:.*|image: ghcr.io/acme/$name:$tag|" "$f"
  printf 'pin:\n  tag: %s\n  weight: %s\n' "$tag" "$((replicas * 10))" >> "$f"
  python3 - "$name" "$tag" "$rid" <<'PY'
import re,sys
name,tag,rid=sys.argv[1:4]
p="deploy/manifest.yaml"
s=open(p).read()
s,n=re.subn(r"(  - service: %s\n    tag: )\S+(\n    releaseId: )\S+" % re.escape(name),
            r"\g<1>%s\g<2>%s" % (tag,rid), s)
assert n==1, name
open(p,"w").write(s)
PY
done
python3 verify.py
