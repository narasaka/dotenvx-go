# Benchmark

What it costs a Go server to get its configuration from a dotenvx-encrypted
file. The benchmark starts a dummy server five ways:

| Setup | How |
| --- | --- |
| plain | the values are already in the environment, unencrypted: the baseline |
| linked | the same, with the server built with dotenvx-go, which it doesn't use |
| run | `dotenvx run -- app`, which keeps dotenvx running beside the server |
| get | `dotenvx get` decrypts the file to JSON and exits, then the server applies the JSON |
| lib | the server decrypts the file itself with dotenvx-go |

It does that with two env files of random values, encrypted by the dotenvx
CLI: `.env.small` (20 values) and `.env.large` (200). For each start it
records the time until `/health` answers, and the memory (PSS) of every
process involved once the server is serving, and how much of it is
anonymous rather than file-backed. The server checks that every
value arrived decrypted before it answers.

## Running it

On Linux, from this directory:

```sh
sh install-dotenvx.sh    # the dotenvx CLI, checked against its release checksums
go run ./cmd/bench       # -runs, -settle and -json are optional; Ctrl-C stops it cleanly
```

`sh env/gen.sh bin/dotenvx` makes new env files. The keys in `env/.env.keys`
protect nothing but those random values.

## Results

dotenvx 1.75.1, Go 1.26.1, AMD Ryzen 7 5800X, Linux 6.8, with nothing else
running dotenvx. Medians of 5 runs.

| File | Setup | Processes | PSS (MiB) | Anonymous (MiB) | By process (MiB) | Until /health (ms) |
| --- | --- | --- | --- | --- | --- | --- |
| `.env.small` (20 values) | plain | 1 | 5.9 | 1.2 | app 5.9 | 2.9 |
| | linked | 1 | 6.6 | 1.3 | app 6.6 | 3.0 |
| | run | 2 | 79.4 | 42.1 | app 5.9, dotenvx 73.5 | 425.8 |
| | get | 1 | 6.1 | 1.3 | app 6.1 | 443.1 |
| | lib | 1 | 6.9 | 1.5 | app 6.9 | 5.2 |
| `.env.large` (200 values) | plain | 1 | 6.0 | 1.3 | app 6.0 | 3.0 |
| | linked | 1 | 6.7 | 1.3 | app 6.7 | 2.9 |
| | run | 2 | 96.8 | 59.4 | app 5.9, dotenvx 90.8 | 2941.3 |
| | get | 1 | 6.4 | 1.4 | app 6.4 | 2942.5 |
| | lib | 1 | 8.2 | 2.8 | app 8.2 | 27.8 |

- `dotenvx run` keeps 74 to 91 MiB beside the server for as long as it runs.
  About 33 MiB of that is file-backed, mostly the pages of the dotenvx
  executable it has loaded, which other dotenvx processes on the same machine
  would share; the rest, 41 to 58 MiB, is its own.
- Both dotenvx setups take 0.43 s (20 values) to 2.9 s (200 values) before the
  server can start. From the two files, each extra value adds about 14 ms.
- Loading the file with dotenvx-go adds about 2 ms (20 values) to 25 ms (200
  values) to startup, against the linked baseline. That includes reading and
  parsing the file as well as decrypting it.
- The server using dotenvx-go is 1 to 2 MiB bigger than the baseline. Linking
  the library adds about 0.7 MiB (linked against plain); loading the file
  adds the rest, most of it anonymous memory.
- Against `dotenvx run`, that's 73 MiB (20 values) to 89 MiB (200 values)
  less in total.
