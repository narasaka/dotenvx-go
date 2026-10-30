# Benchmark

What it costs a Go server to get its configuration from a dotenvx-encrypted
file. The benchmark starts a dummy server four ways:

| Setup | How |
| --- | --- |
| plain | the values are already in the environment, unencrypted: the baseline |
| run | `dotenvx run -- app`, which keeps dotenvx running beside the server |
| get | `dotenvx get` decrypts the file to JSON and exits, then the server applies the JSON |
| lib | the server decrypts the file itself with dotenvx-go |

It does that with two env files of random values, encrypted by the dotenvx
CLI: `.env.small` (20 values) and `.env.large` (200). For each start it
records the time until `/health` answers, and the memory (PSS) of every
process involved once the server is serving. The server checks that every
value arrived decrypted before it answers.

## Running it

On Linux, from this directory:

```sh
sh install-dotenvx.sh    # the dotenvx CLI, checked against its release checksums
go run ./cmd/bench       # -runs, -settle and -json are optional
```

`sh env/gen.sh bin/dotenvx` makes new env files. The keys in `env/.env.keys`
protect nothing but those random values.

## Results

dotenvx 1.75.1, Go 1.26.1, AMD Ryzen 7 5800X, Linux 6.8. Medians of 5 runs.

| File | Setup | Processes | PSS (MiB) | By process (MiB) | Until /health (ms) |
| --- | --- | --- | --- | --- | --- |
| `.env.small` (20 values) | plain | 1 | 5.9 | app 5.9 | 2.9 |
| | run | 2 | 80.3 | app 6.0, dotenvx 74.3 | 439.4 |
| | get | 1 | 6.0 | app 6.0 | 440.2 |
| | lib | 1 | 9.9 | app 9.9 | 9.6 |
| `.env.large` (200 values) | plain | 1 | 5.9 | app 5.9 | 2.9 |
| | run | 2 | 97.5 | app 5.9, dotenvx 91.5 | 2956.0 |
| | get | 1 | 6.4 | app 6.4 | 2941.0 |
| | lib | 1 | 11.7 | app 11.7 | 34.8 |

- `dotenvx run` costs 74 to 92 MiB for as long as the server runs, and both
  dotenvx setups spend 0.4 s to 3 s starting the CLI and decrypting before the
  server can start.
- dotenvx-go decrypts the small file in about 7 ms and the large one in about
  32 ms.
- The server using dotenvx-go is 4 to 6 MiB bigger. Linking the library adds
  0.7 MiB; most of the rest is the secp256k1 library building its
  precomputed table on first use, of which about 1 MiB stays in use and the
  rest is heap Go hasn't yet returned to the operating system.
