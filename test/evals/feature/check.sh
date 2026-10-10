#!/usr/bin/env bash
# pass: tests green, a JSON test exists, --json is well-formed, the table is unchanged
set -u
go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }
grep -qiE 'func Test[A-Za-z0-9_]*json' *_test.go || { echo "no JSON test"; exit 1; }
go run . --json > /tmp/eval-feature-$$.json 2>/dev/null || { echo "--json failed"; exit 1; }
python3 -I - /tmp/eval-feature-$$.json <<'PY' || exit 1
import json, sys
d = json.load(open(sys.argv[1]))
assert isinstance(d, list) and len(d) == 3, "want a 3-item array"
for o in d:
    assert set(o) >= {"name", "qty", "price"}, f"keys: {o}"
    assert isinstance(o["price"], (int, float)) and isinstance(o["qty"], int), f"types: {o}"
assert d[1]["name"] == "gadget" and abs(d[1]["price"] - 19.99) < 1e-9
PY
rm -f /tmp/eval-feature-$$.json
want=$'NAME         QTY    PRICE\nwidget        12     2.50\ngadget         3    19.99\ndoohickey     40     0.75\nTOTAL              119.97'
[ "$(go run . 2>/dev/null)" = "$want" ] || { echo "default table changed"; exit 1; }
