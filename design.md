# Design proposal

## Objective

Implement a simple remote process supervisor that lets its users start, stop and monitor arbitrary commands, as well as retrieving their outputs as streams. A CLI tool will be provided to interact with the server via its gRPC-powered API.

## Assumptions, decisions and tradeoffs
- Given many different implementation options, the most straighforward one will be chosen unless further requirements are provided
- The jobs provided by users are well-intentioned and not malicious, the resource control mechanisms described below act as a safeguard against user/software errors, not targeted attacks
- There will not be any attempts to persist the jobs or recover them on failure
- The job list and their outputs will be held in memory, every attempt to read a stream will start from the beginning
- All the jobs get the same set of resource limits
- Everything contained in this document is a proposal and subject to approval and improvements, the final code may not exactly match this document
- Certificate revocation is considered to be out of scope for this challenge, potential future options could be to add another service providing [CRL](https://en.wikipedia.org/wiki/Certificate_revocation_list) / [OCSP](https://en.wikipedia.org/wiki/Online_Certificate_Status_Protocol).

## Library

The library will expose functions to start a given job, stop it, query its status and get a stream of its standard output and standard error. This is the proposed structure:

```go
type Status struct {
	Status   string
	ExitCode int
}

func NewSupervisor() *Supervisor
	Creates a new supervisor object that will handle the job operations.

func (s *Supervisor) StartJob(cmd string, args ...string) (string, error)
	Starts a new job with the provided command and arguments, using the os/exec standard package. Returns a UUID that will uniquely identify the job in the subsequent operations, or an error if the job could not be started.

func (s *Supervisor) StopJob(id string) error
	If the process has not finished running, it will get killed with os.Process.Kill(). This function will return immediately, as the status can be queried afterwards.

func (s *Supervisor) JobStatus(id string) Status
	Returns a Status struct that contains the status ("Started", "Done", "Stopped") of the job and its exit code (if it corresponds).

func (s *Supervisor) JobStdOut(id string) (io.ReadCloser, error)
	Returns a stream that corresponds to the standard output of the process.

func (s *Supervisor) JobStdErr(id string) (io.ReadCloser, error)
	Returns a stream that corresponds to the standard error of the process.
```

In all cases, an error will be returned if the provided job ID does not exist.

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

message StopResponse {}

enum Status {
    STARTED = 0;
    DONE = 1;
    STOPPED = 2;
}

message StatusResponse {
    Status status = 1;
    int64 exitCode = 2;
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

Any client with a valid certificate (signed by the certificate authority) is authorized to start a new job. To keep things simple, each job is considered to be owned by the user that started it, users are not able to interact in any way through the API with jobs not started by them. The users will be distinguished from each other solely by the authentication mechanism described above (i.e. their certificates provided in the mTLS connections).

No kind of [RBAC](https://en.wikipedia.org/wiki/Role-based_access_control) will be implemented.

## Server

An unsuccessful invocation of `overseer-server` will return a non-zero exit code. Keys and certificates are expected to be in PEM format.

### Usage

`overseer-server [-key PRIVATE-KEY] [-cert SERVER-CERTIFICATE] [-ca CA-CERTIFICATE] [-listen ADDRESS:PORT]`

### Optional flags

`-listen ADDRESS:PORT` Listening address and port. Default: `localhost:9999`.

`-key PRIVATE-KEY` Path to the server private key. Default: `certs/server.key`.

`-cert SERVER-CERTIFICATE` Path to the server certificate. Default: `certs/server.crt`.

`-ca CA-CERTIFICATE` Path to the certificate of the Certificate Authority. Default: `certs/ca.crt`.

## Client

A successful invocation of `overseer-cli` will have a return code of zero, a non-zero value is used for error cases. Keys and certificates are expected to be in PEM format.

### Usage

`overseer-cli [-server ADDRESS:PORT] [-key PRIVATE-KEY] [-cert USER-CERTIFICATE] [-ca CA-CERTIFICATE] ACTION [ARGS...]`

### Optional flags

`-server ADDRESS:PORT` Remote server hostname or IP address and port. Default: `localhost:9999`.

`-key PRIVATE-KEY` Path to the user private key. Default: `certs/user.key`.

`-cert USER-CERTIFICATE` Path to the user certificate. Default: `certs/user.crt`.

`-ca CA-CERTIFICATE` Path to the certificate of the Certificate Authority. Default: `certs/ca.crt`.

### Action flags

Only one action is allowed per invocation.

`-start PATH [ARGS...]` Connects to the job server at the IP/hostname given in `ADDRESS`, using the port provided in `PORT`, then starts the job at the given path (`PATH`) in the server and the arguments that follow (`ARGS`). A `JOB-ID` will be returned to uniquely identify the started job, or an error message if the execution failed.

`-stop JOB-ID` Stops the job identified by `JOB-ID` and returns its exit code or an error if the provided job did not exist. It must be used to release the resources of the system.

`-status JOB-ID` Returns the current state (Started, Done or Stopped) of the job identified by `JOB-ID` and its exit code if it corresponds, or an error if the provided job did no exist.

`-stdout JOB-ID` Writes the standard output of the given job to the standard output of this process, or returns an error if the provided job did no exist.

`-stderr JOB-ID` Writes the standard error of the given job to the standard output of this process, or returns an error if the provided job did no exist.

### Example session

```
$ overseer-cli -server localhost:8888 -key user.key -cert user.crt -start echo Hello
UNIQUE-JOB-ID

$ overseer-cli -server localhost:8888 -key user.key -cert user.crt -status UNIQUE-JOB-ID
Done = 0

$ overseer-cli -server localhost:8888 -key user.key -cert user.crt -stdout UNIQUE-JOB-ID
Hello
```

## Resource control

After evaluating the alternatives (`systemd-run`, `cpulimit`, `nice`, `ionice`, `cgroup`, `prlimit`, `setrlimit`) and discussing with the evaluation team, [cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) was chosen. In order for this to work, before starting a new job, the server process will run itself, set the needed cgroup resource controls and then call [`unix.Exec()`](https://pkg.go.dev/golang.org/x/sys/unix#Exec) to start the job. The external package [golang.org/x/sys/unix](https://pkg.go.dev/golang.org/x/sys/unix) will be used because the `syscall` package of the standard library is deprecated. The cgroup controllers to be used are: `cpu`, `io`, `memory`.
