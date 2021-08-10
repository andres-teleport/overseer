package supervisor

import (
	"bytes"
	"io"
	"testing"
)

func TestFailedStart(t *testing.T) {
	sup := NewSupervisor()

	if jobID, err := sup.StartJob("/tmp"); len(jobID) > 0 || err == nil {
		t.Fail()
	}
}

func TestStartStopStatus(t *testing.T) {
	sup := NewSupervisor()

	jobID, err := sup.StartJob("sleep", "999")
	if err != nil {
		t.Fatal(err)
	}

	status, err := sup.JobStatus(jobID)
	if err != nil {
		t.Fatal(err)
	} else if status.Status != StatusStarted {
		t.Errorf("StatusStarted expected, %d got", status.Status)
	}

	if err := sup.StopJob(jobID); err != nil {
		t.Fatal(err)
	}

	if err := sup.StopJob(jobID); err != ErrJobFinished {
		t.Errorf("expected '%s', got '%s'", ErrJobFinished, err)
	}

	status, err = sup.JobStatus(jobID)
	if err != nil {
		t.Fatal(err)
	} else if status.Status != StatusStopped {
		t.Errorf("StatusStopped expected, %d got", status.Status)
	}
}

func TestStartStopTwice(t *testing.T) {
	sup := NewSupervisor()

	jobID, err := sup.StartJob("sleep", "1")
	if err != nil {
		t.Fatal(err)
	}

	rd, err := sup.JobStdOut(jobID)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, rd)

	status, err := sup.JobStatus(jobID)
	if err != nil {
		t.Fatal(err)
	} else if status.Status != StatusDone {
		t.Errorf("StatusDone expected, %d got", status.Status)
	}

	if err := sup.StopJob(jobID); err != ErrJobFinished {
		t.Errorf("expected '%s', got '%s'", ErrJobFinished, err)
	}

	if err := sup.StopJob(jobID); err != ErrJobFinished {
		t.Errorf("expected '%s', got '%s'", ErrJobFinished, err)
	}

	status, err = sup.JobStatus(jobID)
	if err != nil {
		t.Fatal(err)
	} else if status.Status != StatusDone {
		t.Errorf("StatusDone expected, %d got", status.Status)
	}
}

func TestStdOut(t *testing.T) {
	sup := NewSupervisor()
	testString := "hello world"

	jobID, err := sup.StartJob("echo", testString)
	if err != nil {
		t.Fatal(err)
	}

	rd, err := sup.JobStdOut(jobID)
	if err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	out = bytes.TrimSpace(out)

	if !bytes.Equal(out, []byte(testString)) {
		t.Errorf("expected '%s', got '%s'", testString, string(out))
	}
}

func TestStdErr(t *testing.T) {
	sup := NewSupervisor()
	testString := "hello world"

	jobID, err := sup.StartJob("sh", "-c", "/bin/echo "+testString+" >&2")
	if err != nil {
		t.Fatal(err)
	}

	// StdErr
	rd, err := sup.JobStdErr(jobID)
	if err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	out = bytes.TrimSpace(out)

	if !bytes.Equal(out, []byte(testString)) {
		t.Errorf("expected '%s', got '%s'", testString, string(out))
	}

	// StdOut
	rd, err = sup.JobStdOut(jobID)
	if err != nil {
		t.Fatal(err)
	}

	out, err = io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	out = bytes.TrimSpace(out)

	if len(out) > 0 {
		t.Errorf("expected '', got '%s'", string(out))
	}
}

func TestUnknownJobID(t *testing.T) {
	sup := NewSupervisor()

	if _, err := sup.JobStatus("fake-id"); err != ErrUnknownJobID {
		t.Errorf("expected '%s', got '%s'", ErrUnknownJobID, err)
	}
}
