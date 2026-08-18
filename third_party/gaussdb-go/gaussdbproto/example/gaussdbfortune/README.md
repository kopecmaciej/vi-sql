# gaussdbfortune

gaussdbfortune is a mock GaussDB server that responds to every query with a fortune.

## Installation

Install `fortune` and `cowsay`. They should be available in any Unix package manager (apt, yum, brew, etc.)

```
go get -u github.com/HuaweiCloudDeveloper/gaussdb-go/example/gaussdbfortune
```

## Usage

```
$ gaussdbfortune
```

By default gaussdbfortune listens on 127.0.0.1:15432 and responds to queries with `fortune | cowsay -f elephant`. These are
configurable with the `listen` and `response-command` arguments respectively.

While `gaussdbfortune` is running connect to it with `psql`.

```
$ psql -h 127.0.0.1 -p 15432
Timing is on.
Null display is "∅".
Line style is unicode.
psql (11.5, server 0.0.0)
Type "help" for help.

jack@127.0.0.1:15432 jack=# select foo;
                   fortune
─────────────────────────────────────────────
  _________________________________________ ↵
 / Ships are safe in harbor, but they were \↵
 \ never meant to stay there.              /↵
  ----------------------------------------- ↵
  \     /\  ___  /\                         ↵
   \   // \/   \/ \\                        ↵
      ((    O O    ))                       ↵
       \\ /     \ //                        ↵
        \/  | |  \/                         ↵
         |  | |  |                          ↵
         |  | |  |                          ↵
         |   o   |                          ↵
         | |   | |                          ↵
         |m|   |m|                          ↵

(1 row)

Time: 28.161 ms
```
