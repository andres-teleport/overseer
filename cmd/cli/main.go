package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/andres-teleport/overseer/api"
	"github.com/andres-teleport/overseer/api/client"
)

var (
	errNoActionProvided = errors.New("no action was provided")
)

func main() {
	log.SetFlags(0)

	// Optional flags
	var server, key, cert, ca string
	flag.StringVar(&server, "server", "localhost:9999", "remote server address")
	flag.StringVar(&key, "key", "certs/user.key", "path to the private key")
	flag.StringVar(&cert, "cert", "certs/user.crt", "path to the certificate")
	flag.StringVar(&ca, "ca", "certs/ca.crt", "path to the certificate of the Certificate Authority")

	// Action flags
	var startCmd, stopJobID, statusJobID, stdOutJobID, stdErrJobID string
	flag.StringVar(&startCmd, "start", "", "description")
	flag.StringVar(&stopJobID, "stop", "", "description")
	flag.StringVar(&statusJobID, "status", "", "description")
	flag.StringVar(&stdOutJobID, "stdout", "", "description")
	flag.StringVar(&stdErrJobID, "stderr", "", "description")
	flag.Parse()

	// TODO: return error if more than one action is supplied

	cli, err := client.NewClient(server, key, cert, ca)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	switch {
	case len(startCmd) > 0:
		var jobID string
		if jobID, err = cli.Start(ctx, startCmd, flag.Args()...); err == nil {
			fmt.Println(jobID)
		}
	case len(stopJobID) > 0:
		err = cli.Stop(ctx, stopJobID)
	case len(statusJobID) > 0:
		var status *api.StatusResponse
		if status, err = cli.Status(ctx, statusJobID); err == nil {
			if status.Status != api.Status_STARTED {
				fmt.Println(status.Status, "=", status.ExitCode)
			} else {
				fmt.Println(status.Status)
			}
		}
	case len(stdOutJobID) > 0:
		var rd *io.PipeReader
		rd, err = cli.StdOut(ctx, stdOutJobID)
		if err == nil {
			_, err = io.Copy(os.Stdout, rd)
			_ = rd.Close()
		}
	case len(stdErrJobID) > 0:
		var rd *io.PipeReader
		rd, err = cli.StdErr(ctx, stdErrJobID)
		if err == nil {
			_, err = io.Copy(os.Stderr, rd)
			_ = rd.Close()
		}
	default:
		err = errNoActionProvided
	}

	if err != nil {
		log.Fatal(err)
	}
}
