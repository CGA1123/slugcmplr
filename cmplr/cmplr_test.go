package cmplr_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bissyio/slugcmplr/cmplr"
	"github.com/bissyio/slugcmplr/procfile"
)

func Test_EscapeReleaseTask(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "Procfile")
	if err != nil {
		t.Fatalf("failed creating tempfile: %v", err)
	}

	path, err := filepath.Abs(f.Name())
	if err != nil {
		t.Fatalf("failed to get filepath for tempfile: %v", err)
	}

	contents := `web: bin/server
release: bin/migrate
worker: bin/worker
`
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("error writing test contents to file: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	if err := cmplr.EscapeReleaseTask(path); err != nil {
		t.Fatalf("error EscapeReleaseTask: %v", err)
	}

	f, err = os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file (%v): %v", path, err)
	}

	procf, err := procfile.Read(f)
	if err != nil {
		t.Fatalf("error reading procfile: %v", err)
	}

	expected := map[string]string{"web": "bin/server",
		"release": "[ ! -z $SLUGCMPLR ] || bin/migrate",
		"worker":  "bin/worker"}

	for proc, cmd := range expected {
		if actual, _ := procf.Entrypoint(proc); actual != cmd {
			t.Fatalf("expected %v got %v", cmd, actual)
		}
	}
}
