# README

## Prerequisites

- Go v1.16
- Ubuntu 20.04 or later

## Troubleshooting

### Cgroup2

The `systemd.unified_cgroup_hierarchy=1` boot parameter may need to be added to
the `GRUB_CMDLINE_LINUX_DEFAULT` variable in `/etc/default/grub` and then
`sudo update-grub` must be run. Otherwise cgroup v2 won't have access to the
needed cgroup controllers.


### Flag collision

If the job command requires command line flags these will interfere with the CLI
flag parser, a workaround is to pass the flags after a `--`, so the command will
look like this:

```
./cli [CLI_FLAGS] -start CMD -- CMD_FLAGS_AND_ARGS
```

Example:

```
./cli -start ls -- -lh /
```

### Running tests

The tests will need to run under a privileged user (i.e. using `sudo`) unless
the current user has enough permissions to modify the cgroup2 filesystem.

## Instructions

### Running the tests

```
./test.sh
```

### Building the binaries

```
./build.sh
```

### Running the CLI tool

```
cd bin
./cli
```

For more information about usage please look at the
[design document](https://github.com/andres-teleport/overseer/blob/main/design.md).
Default certificates for testing are provided under `bin/certs`.

### Running the server

```
cd bin
./server
```

For more information about usage please look at the
[design document](https://github.com/andres-teleport/overseer/blob/main/design.md).
Default certificates for testing are provided under `bin/certs`.
