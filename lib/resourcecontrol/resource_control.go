package resourcecontrol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"

	"golang.org/x/sys/unix"
)

// NOTE: calling log.Fatal() inside a library is usually considered bad
// practice, in this case it only happens in the child process so the library
// user will not be affected.

const (
	execEnvVar      = "OVERSEER_EXEC"
	cpuMaxEnvVar    = "OVERSEER_CPU_MAX"
	memMaxEnvVar    = "OVERSEER_MEM_MAX"
	ioMaxRbpsEnvVar = "OVERSEER_IO_MAX_RBPS"
	ioMaxWbpsEnvVar = "OVERSEER_IO_MAX_WBPS"

	controlSubtree = "overseer"
)

var (
	errCgroupNotMounted = errors.New("cgroup2 is not mounted")
)

// TODO: add more limits
type ResourceLimits struct {
	CPUMax    string
	MemMax    string
	IOMaxRbps string
	IOMaxWbps string
}

// init will look for a special environment variable that, if present, will
// signal the need to set up the resource limits and call unix.Exec() to start
// the limited command
func init() {
	if os.Getenv(execEnvVar) == execEnvVar {
		// Unset custom environment variables
		for _, v := range []string{execEnvVar, cpuMaxEnvVar, memMaxEnvVar, ioMaxRbpsEnvVar, ioMaxWbpsEnvVar} {
			os.Unsetenv(v)
		}

		setResourceLimits()
		// TODO: drop privileges

		if err := unix.Exec(os.Args[1], os.Args[1:], os.Environ()); err != nil {
			log.Fatal(err)
		}
	}
}

func getCgroupRootPath() (string, error) {
	r, err := os.Open("/proc/self/mounts")
	if err != nil {
		return "", err
	}
	defer r.Close()

	var mountType, path, d string
	for eof := false; !eof; {
		_, err := fmt.Fscanf(r, "%s %s %s %s %s %s\n", &mountType, &path, &d, &d, &d, &d)

		if err == io.EOF {
			eof = true
		} else if err != nil {
			return "", err
		}

		if mountType == "cgroup2" {
			return path, nil
		}
	}

	// TODO: try to mount it before giving up

	return "", errCgroupNotMounted
}

func getBlockDevs() ([]string, error) {
	basePath := "/sys/block"

	ds, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	var rs []string
	for _, d := range ds {
		dev, err := os.ReadFile(path.Join(basePath, d.Name(), "dev"))
		if err != nil {
			return nil, err
		}
		rs = append(rs, string(bytes.TrimSpace(dev)))
	}

	return rs, nil
}

func setResourceLimits() {
	cgroupPath, err := getCgroupRootPath()
	if err != nil {
		log.Fatal(err)
	}

	cgroupPath = path.Join(cgroupPath, controlSubtree)

	if err := os.Mkdir(cgroupPath, 0755); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	// Set CPU limits
	if v, ok := os.LookupEnv(cpuMaxEnvVar); ok {
		if err := os.WriteFile(path.Join(cgroupPath, "cpu.max"), []byte(v), 0700); err != nil {
			log.Fatal(err)
		}
	}

	// Set memory limits
	if v, ok := os.LookupEnv(memMaxEnvVar); ok {
		if err := os.WriteFile(path.Join(cgroupPath, "memory.max"), []byte(v), 0700); err != nil {
			log.Fatal(err)
		}
	}

	// Set IO limits
	ioLimitString := ""
	if v, ok := os.LookupEnv(ioMaxRbpsEnvVar); ok {
		ioLimitString += " rbps=" + v
	}

	if v, ok := os.LookupEnv(ioMaxWbpsEnvVar); ok {
		ioLimitString += " wbps=" + v
	}

	if ioLimitString != "" {
		blkMajorMinors, err := getBlockDevs()
		if err != nil {
			log.Fatal(err)
		}

		for _, b := range blkMajorMinors {
			if err := os.WriteFile(path.Join(cgroupPath, "io.max"), []byte(b+ioLimitString), 0700); err != nil {
				log.Fatal(err)
			}
		}
	}

	// Add own PID to the cgroup
	if err := os.WriteFile(path.Join(cgroupPath, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700); err != nil {
		log.Fatal(err)
	}
}

// Command takes the given name and args and returns a command with resource
// limits enforced
func Command(limits ResourceLimits, name string, args ...string) *exec.Cmd {
	os.Setenv(execEnvVar, execEnvVar)

	cmd := exec.Command("/proc/self/exe", append([]string{name}, args...)...)
	cmd.Env = append(os.Environ(),
		execEnvVar+"="+execEnvVar,
		cpuMaxEnvVar+"="+limits.CPUMax,
		memMaxEnvVar+"="+limits.MemMax,
		ioMaxRbpsEnvVar+"="+limits.IOMaxRbps,
		limits.IOMaxWbps+"="+limits.IOMaxWbps,
	)

	return cmd
}
