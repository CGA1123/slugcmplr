package procfile_test

import (
	"strings"
	"testing"

	"github.com/cga1123/slugcmplr/procfile"
)

func Contain(t *testing.T, expected, actual []string) {
	m := make(map[string]struct{}, len(actual))
	for _, v := range actual {
		m[v] = struct{}{}
	}

	missing := []string{}

	for _, expect := range expected {
		if _, ok := m[expect]; !ok {
			missing = append(missing, expect)
		}
	}

	if len(missing) != 0 {
		t.Fatalf("Expected %v to contain %v. %v were missing.", actual, expected, missing)
	}
}

func Test_ProcfileInMemory(t *testing.T) {
	t.Parallel()

	p := procfile.New()

	if _, ok := p.Entrypoint("web"); ok {
		t.Fatalf("Entrypoint erroneous ok returned! %v", p)
	}

	p.Add("web", "bin/server")
	if !p.Defined("web") {
		t.Fatalf("Add did not persist entry: %v", p)
	}

	entrypoint, ok := p.Entrypoint("web")
	if !ok {
		t.Fatalf("Entrypoint was not ok! %v", p)
	}

	if entrypoint != "bin/server" {
		t.Fatalf("Entrypoint did not match expected: %v", p)
	}

	p.Remove("web")
	if p.Defined("web") {
		t.Fatalf("Remove did not remove process! %v", p)
	}
}

func Test_ProcfileProcesses(t *testing.T) {
	t.Parallel()

	p := procfile.New()

	if len(p.Processes()) != 0 {
		t.Fatalf("Processes returned non-empty slice! %v", p)
	}

	p.Add("web", "bin/server")
	procs := p.Processes()

	if len(procs) != 1 || procs[0] != "web" {
		t.Fatalf("Processes did not contain only 'web'! %v", p)
	}

	p.Add("worker", "bundle exec sidekiq")
	twoProcs := p.Processes()

	if len(twoProcs) != 2 {
		t.Fatalf("Only expected 2 processes! %v", p)
	}

	Contain(t, []string{"web", "worker"}, twoProcs)
}

func Test_Read(t *testing.T) {
	t.Parallel()

	valid := `web: bin/server
worker: bundle exec sidekiq -c config/sidekiq.yml
cron: bin/scheduler`

	procf, err := procfile.Read(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("unexpected error when reading procfile: %v", err)
	}

	Contain(t, []string{"web", "worker", "cron"}, procf.Processes())

	expected := map[string]string{
		"web":    "bin/server",
		"worker": "bundle exec sidekiq -c config/sidekiq.yml",
		"cron":   "bin/scheduler",
	}

	for proc, cmd := range expected {
		actual, ok := procf.Entrypoint(proc)
		if !ok {
			t.Fatalf("expected %v to be in procfile %v", proc, procf)
		}

		if actual != cmd {
			t.Fatalf("expected %v to have command %v, had %v", proc, cmd, actual)
		}
	}
}

func Test_Write(t *testing.T) {
	t.Parallel()

	procf := procfile.New()
	procf.Add("web", "bin/server")
	procf.Add("worker", "bin/worker")
	procf.Add("scheduler", "bin/scheduler")

	expected := map[string]string{
		"web":       "bin/server",
		"worker":    "bin/worker",
		"scheduler": "bin/scheduler"}

	builder := &strings.Builder{}

	procf.Write(builder)

	actualProcf, err := procfile.Read(strings.NewReader(builder.String()))
	if err != nil {
		t.Fatalf("err reading actual procfile: %v", err)
	}

	for proc, cmd := range expected {
		if actualCmd, _ := actualProcf.Entrypoint(proc); actualCmd != cmd {
			t.Fatalf("expected %v, got %v. (%v)", cmd, actualCmd, proc)
		}
	}

}
