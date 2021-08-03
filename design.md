# Design proposal

## Objective

Implement a simple remote process supervisor that lets its users start, stop and monitor arbitrary commands, as well as retrieving their outputs as streams. A CLI tool will be provided to interact with the server via its gRPC-powered API.

## Assumptions, decisions and tradeoffs
- Given many different implementation options, the most straighforward one will be chosen unless further requirements are provided
- The jobs provided by users are well-intentioned and not malicious, the resource control mechanisms described below act as a safeguard against user/software errors, not targeted attacks
- There will not be any attempts to persist the jobs or recover them on failure
- The output from the jobs will be held in memory until consumed or discarded
- All the jobs get the same set of resource limits
- Everything contained in this document is a proposal and subject to approval and improvements, the final code may not exactly match this document

## Library

The library will expose functions to start a given job, stop it, query its status and get a stream of its standard output and standard error. This is the proposed structure:

```go
func NewSupervisor() *Supervisor
	Creates a new supervisor object that will handle the job operations.

func (s *Supervisor) StartJob(cmd string, args ...string) (string, error)
	Starts a new job with the provided command and arguments, using the os/exec standard package.

func (s *Supervisor) StopJob(id string) (int, error)
	Must be called to free the resources allocated by the supervisor for this process. If the process has not finished running, it will get killed with os.Process.Kill().

func (s *Supervisor) JobStatus(id string) string
	Returns a string with the status of the given job: "Started" or "Done".

func (s *Supervisor) JobStdOut(id string) (io.ReadCloser, error)
	Returns a stream that corresponds to the standard output of the process.

func (s *Supervisor) JobStdErr(id string) (io.ReadCloser, error)
	Returns a stream that corresponds to the standard error of the process.
```

## API

The API will use the gRPC framework for communication and Protocol Buffers as the serialization format.

### Protocol definition

```protobuf
syntax = "proto3";

package overseer;

message Job {
    string command = 1;
    repeated string arguments = 2;
}

message JobID {
    string id = 1;
}

message StopResponse {
    int64 exitCode = 1;
}

enum Status {
    STARTED = 0;
    DONE = 1;
}

message StatusResponse {
    Status status = 1;
}

message OutputChunk {
    bytes output = 1;
}

service JobworkerService {
    rpc Start(Job) returns (JobID) {}
    rpc Stop(JobID) returns (StopResponse) {}
    rpc Status(JobID) returns (StatusResponse) {}
    rpc StdOut(JobID) returns (stream OutputChunk) {}
    rpc StdErr(JobID) returns (stream OutputChunk) {}
}
```

### Authentication
[mTLS](https://en.wikipedia.org/wiki/Mutual_authentication#mTLS) will be used, each user will have its own [X.509 certificate](https://en.wikipedia.org/wiki/X.509) ([PEM format](https://en.wikipedia.org/wiki/Privacy-Enhanced_Mail)), and the server too. Following the latest security standards, only [TLS v1.3](https://en.wikipedia.org/wiki/Transport_Layer_Security#TLS_1.3) with the following ciphersuites will be allowed:

- `TLS_AES_128_GCM_SHA256`
- `TLS_AES_256_GCM_SHA384`
- `TLS_CHACHA20_POLY1305_SHA256`

To simplify the testing process some pregenerated certificates will be provided, these are deemed unsafe for other purposes as they have been publicly exposed. New and safe certificates (RSA 2048-bit) can be generated with the following commands, using the [certstrap](https://github.com/square/certstrap) (or any similar) tool:

#### Initialize a new certificate authority (to sign/verify server/client certificates)

```
$ certstrap init --common-name ca
Enter passphrase (empty for no passphrase):
Enter same passphrase again:
Created out/ca.key
Created out/ca.crt
Created out/ca.crl
```

Only `ca.crt` is needed by the server or client to verify signatures. Do not distribute the other files.

#### Request a new certificate for a client

```
$ certstrap request-cert --common-name user
Enter passphrase (empty for no passphrase):
Enter same passphrase again:
Created out/user.key
Created out/user.csr
```

`user.key` should be held only by the client and should be not distributed otherwise.

#### Request a new certificate for a server

```
$ certstrap request-cert --domain localhost
Enter passphrase (empty for no passphrase):
Enter same passphrase again:
Created out/localhost.key
Created out/localhost.csr
```

`localhost.key` should be held only by the server and should not be distributed otherwise.

#### Sign a requested certificate so it can be trusted by the server

```
$ certstrap sign user --CA ca
Created out/user.crt from out/user.csr signed by out/ca.key
```

### Authorization

The client will check the server provided certificate against his own known server certificate (provided by the sysadmin). The server will have a directory with the authorized client certificates. This is done so revoking access becomes possible without adding another service providing [CRL](https://en.wikipedia.org/wiki/Certificate_revocation_list) / [OCSP](https://en.wikipedia.org/wiki/Online_Certificate_Status_Protocol) or other mechanisms.

To keep things simple, each job is considered to be owned by the user that started it, users are not able to interact in any way through the API with jobs not started by them.

No kind of [RBAC](https://en.wikipedia.org/wiki/Role-based_access_control) will be implemented.

## Server

An unsuccessful invocation of `overseer-server` will return a non-zero exit code.

### Usage

`overseer-server [-key PRIVATE-KEY] [-authorized-users USER-CERTIFICATE-DIR] [-listen ADDRESS:PORT]`

### Optional flags

`-key PRIVATE-KEY` Path to the server private key in PEM format. Default: `certs/server.key`.

`-authorized-users USER-CERTIFICATE-DIR` Path to the directory containing the authorized certificates. Default: `certs/authorized`.

`-listen ADDRESS:PORT` Listening address and port. Default: `localhost:9999`.

## Client

A successful invocation of `overseer-cli` will have a return code of zero, a non-zero value is used for error cases.

### Usage

`overseer-cli [-server ADDRESS:PORT] [-key PRIVATE-KEY] [-cert SERVER-CERTIFICATE] ACTION [ARGS...]`

### Optional flags

`-server ADDRESS:PORT` Remote server hostname or IP address and port. Default: `localhost:9999`.

`-key PRIVATE-KEY` Path to the user private key in PEM format. Default: `certs/user.key`.

`-cert SERVER-CERTIFICATE` Path to the remote server certificate in PEM format. Default: `certs/server.crt`.

### Action flags

Only one action is allowed per invocation.

`-start PATH [ARGS...]` Connects to the job server at the IP/hostname given in `ADDRESS`, using the port provided in `PORT`, then starts the job at the given path (`PATH`) in the server and the arguments that follow (`ARGS`). A `JOB-ID` will be returned to uniquely identify the started job, or an error message if the execution failed.

`-stop JOB-ID` Stops the job identified by `JOB-ID` and returns its exit code or an error if the provided job did not exist. It must be used to release the resources of the system.

`-status JOB-ID` Returns the current state (Started or Done) of the job identified by `JOB-ID` or an error if the provided job did no exist.

`-stdout JOB-ID` Writes the standard output of the given job to the standard output of this process, or returns an error if another instances of this process is already connected.

`-stderr JOB-ID`	Writes the standard error of the given job to the standard output of this process, or returns an error if another instance of this process is already connected.

### Example session

```
$ overseer-cli -server localhost:8888 -key my-key.key -cert server.crt -start echo Hello
UNIQUE-JOB-ID

$ overseer-cli -server localhost:8888 -key my-key.key -cert server.crt -status UNIQUE-JOB-ID
Done

$ overseer-cli -server localhost:8888 -key my-key.key -cert server.crt -stdout UNIQUE-JOB-ID
Hello

$ overseer-cli -server localhost:8888 -key my-key.key -cert server.crt -stop UNIQUE-JOB-ID
0
```


## Resource control

There are many ways to control the resources on a Linux system, alternatives considered were: `systemd-run`, `cpulimit`, `nice`, `ionice`,`cgroups`; but the `prlimit()` system call was chosen because it satisfies the requirements (being a system call and being able to enforce all the necessary limits). The proposed solution is to start the given job and then set the limits, the drawback being that the new process could exceed the resources before the limits are set, another drawback is that there is no reliable way to tell if a process exceeded the limits and got killed or not.

The external package [golang.org/x/sys/unix](https://pkg.go.dev/golang.org/x/sys/unix) will be used because the `syscall` package of the standard library is deprecated.

The limits to be used are:

- `RLIMIT_AS`: virtual memory
- `RLIMIT_CPU`: CPU time
- `RLIMIT_DATA`: data and heap size
- `RLIMIT_FSIZE`: size of the files created by the process
- `RLIMIT_NOFILE`: number of files opened by the process
- `RLIMIT_STACK`: stack size

For a more detailed explanation of each limit see here: https://linux.die.net/man/2/prlimit.

If time allows it (and is deemed necessary) an alternative implementation will be attempted, which is to `fork()` the server process, set the resource limits in place using the `setrlimit` system call and then `exec()` to the given job, avoiding the race condition described above. This would be straightforward to do in other languages such as C, but Go makes it more complicated as it doesn't expose `fork()` directly because it interferes with its goroutine scheduling system if not done within some constraints.
