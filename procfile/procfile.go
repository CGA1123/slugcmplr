package procfile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
)

// Procfile contains the definition of a Procfile.
type Procfile map[string]string

var regex = regexp.MustCompile(`^(?P<process>[a-zA-Z0-9]+): (?P<command>.*)$`)

// New creates a new Procfile in-memory.
func New() Procfile {
	return map[string]string{}
}

// Add adds a new entry to the Procfile, overwriting any previous entry for the
// same process.
func (p Procfile) Add(process, entrypoint string) Procfile {
	p[process] = entrypoint

	return p
}

// Remove removes any entry in the Procfile for the given process.
func (p Procfile) Remove(process string) Procfile {
	delete(p, process)

	return p
}

// Entrypoint returns the entrypoint for the given process as defined in the
// Procfile.
func (p Procfile) Entrypoint(process string) (string, bool) {
	cmd, ok := p[process]

	return cmd, ok
}

// Defined checks whether the given process is defined (has an entrypoint) in
// the Procfile.
func (p Procfile) Defined(process string) bool {
	_, ok := p.Entrypoint(process)

	return ok
}

// Processes returns the list of all defined processes in the Procfile.
func (p Procfile) Processes() []string {
	procs := make([]string, 0, len(p))

	for k := range p {
		procs = append(procs, k)
	}

	return procs
}

// Write writes the Procfile to the given io.Writer.
//
// Write does not call Close() on out, the caller is expected to do so.
func (p Procfile) Write(out io.Writer) (int, error) {
	n := 0

	for proc, cmd := range p {
		line := proc + ": " + cmd + "\n"

		written, err := out.Write([]byte(line))
		n += written

		if err != nil {
			return n, err
		}
	}

	return n, nil
}

// Read reads and parses a Procfile from the given io.Reader.
//
// Read does not call Close() on in, the caller is expected to do so.
func Read(in io.Reader) (Procfile, error) {
	scanner := bufio.NewScanner(in)
	procf := New()

	for scanner.Scan() {
		line := scanner.Text()
		result := capture(regex, line)

		if len(result) != 2 {
			return procf, fmt.Errorf("invalid Procfile line: %v", line)
		}

		process, command := result["process"], result["command"]
		procf.Add(process, command)
	}

	return procf, scanner.Err()
}

func capture(r *regexp.Regexp, s string) map[string]string {
	m := map[string]string{}
	names := r.SubexpNames()
	for _, match := range r.FindAllStringSubmatch(s, -1) {
		for i, submatch := range match {
			name := names[i]
			if name == "" {
				continue
			}

			m[name] = submatch
		}
	}

	return m
}
