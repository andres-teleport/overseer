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

### Running tests

The tests will need to run under a privileged user (i.e. using `sudo`) unless
the current user has enough permissions to modify the cgroup2 filesystem.

## Instructions

- For running the tests: `./test.sh`
