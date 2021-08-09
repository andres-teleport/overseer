package resourcecontrol

import (
	"bytes"
	"os"
	"path"
	"testing"
)

func TestSetResourceLimits(t *testing.T) {
	cgroupPath, err := getCgroupRootPath()
	if err != nil {
		t.Fatal(err)
	}

	limits := ResourceLimits{
		CPUMax:    "20000 100000",
		MemMax:    "8388608",
		IOMaxRbps: "1111",
		IOMaxWbps: "3333",
	}

	for _, ev := range genLimitsEnvVars(limits) {
		os.Setenv(ev.name, ev.value)
	}

	if err := setResourceLimits(); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(path.Join(cgroupPath, controlSubtree, "cpu.max"))
	out = bytes.TrimSpace(out)
	if err != nil {
		t.Error(err)
	} else if !bytes.Equal(out, []byte(limits.CPUMax)) {
		t.Errorf("expected '%s', got '%s'", limits.CPUMax, string(out))
	}

	out, err = os.ReadFile(path.Join(cgroupPath, controlSubtree, "memory.max"))
	out = bytes.TrimSpace(out)
	if err != nil {
		t.Error(err)
	} else if !bytes.Equal(out, []byte(limits.MemMax)) {
		t.Errorf("expected '%s', got '%s'", limits.MemMax, string(out))
	}

	// TODO: test IO limits

	limits = ResourceLimits{
		CPUMax:    "max 100000",
		MemMax:    "max",
		IOMaxRbps: "max",
		IOMaxWbps: "max",
	}

	for _, ev := range genLimitsEnvVars(limits) {
		os.Setenv(ev.name, ev.value)
	}

	if err := setResourceLimits(); err != nil {
		t.Error(err)
		t.FailNow()
	}

	out, err = os.ReadFile(path.Join(cgroupPath, controlSubtree, "cpu.max"))
	out = bytes.TrimSpace(out)
	if err != nil {
		t.Error(err)
	} else if !bytes.Equal(out, []byte(limits.CPUMax)) {
		t.Errorf("expected '%s', got '%s'", limits.CPUMax, string(out))
	}

	out, err = os.ReadFile(path.Join(cgroupPath, controlSubtree, "memory.max"))
	out = bytes.TrimSpace(out)
	if err != nil {
		t.Error(err)
	} else if !bytes.Equal(out, []byte(limits.MemMax)) {
		t.Errorf("expected '%s', got '%s'", limits.MemMax, string(out))
	}

	// TODO: test IO limits
}

func TestEcho(t *testing.T) {
	limits := ResourceLimits{
		CPUMax:    "max 100000",
		MemMax:    "max",
		IOMaxRbps: "max",
		IOMaxWbps: "max",
	}

	helloString := "hello world"
	cmd := Command(limits, "echo", helloString)

	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	out = bytes.TrimSpace(out)

	if !bytes.Equal(out, []byte(helloString)) {
		t.Errorf("expected '%s', got '%s'", helloString, string(out))
	}
}
