#!/usr/bin/env bash
# pass: valid conversions unchanged; bad numbers, bad units, missing values and
# temperatures below absolute zero exit non-zero
set -u
go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }
go build -o /tmp/eval-temp-$$ . || { echo "build fails"; exit 1; }
t=/tmp/eval-temp-$$
trap 'rm -f $t' EXIT
[ "$($t -to f 100)" = "212.0" ] || { echo "-to f 100 changed"; exit 1; }
[ "$($t -to c 212)" = "100.0" ] || { echo "-to c 212 changed"; exit 1; }
[ "$($t -to f -- -40)" = "-40.0" ] || { echo "-to f -- -40 changed"; exit 1; }
for args in "-to f abc" "-to f" "-to x 5" "-to f -- -300" "-to c -- -500"; do
  if $t $args >/dev/null 2>&1; then echo "accepted invalid input: $args"; exit 1; fi
done
