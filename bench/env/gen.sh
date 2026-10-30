#!/bin/sh
# Writes the dummy env files the benchmark loads, encrypted by the dotenvx
# CLI itself: .env.small (20 values, about what a small service has) and
# .env.large (200 values). The values are random and the keys in .env.keys
# protect nothing; they exist only for the benchmark.
#
#   sh gen.sh path/to/dotenvx
set -eu
dotenvx=$1
cd "$(dirname "$0")"
rm -f .env.small .env.large .env.keys
for spec in small:20 large:200; do
  name=${spec%%:*} count=${spec#*:}
  i=1
  while [ "$i" -le "$count" ]; do
    printf 'BENCH_VALUE_%03d="%s"\n' "$i" "$(head -c 36 /dev/urandom | base64 | tr -d '\n')"
    i=$((i + 1))
  done > ".env.$name"
  CI=1 "$dotenvx" encrypt -f ".env.$name" -fk .env.keys >/dev/null
done
