package procfile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
)

type Procfile map[string]string

var Regex = regexp.MustCompile(`^(?P<process>[a-zA-Z0-9]+): (?P<command>.*)$`)

func New() Procfile {
	return map[string]string{}
}

func (p Procfile) Add(process, entrypoint string) Procfile {
	p[process] = entrypoint

	return p
}

func (p Procfile) Remove(process string) Procfile {
	delete(p, process)

	return p
}

func (p Procfile) Entrypoint(process string) (string, bool) {
	cmd, ok := p[process]

	return cmd, ok
}

func (p Procfile) Defined(process string) bool {
	_, ok := p.Entrypoint(process)

	return ok
}

func (p Procfile) Processes() []string {
	procs := make([]string, 0, len(p))

	for k := range p {
		procs = append(procs, k)
	}

	return procs
}

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

func Read(in io.Reader) (Procfile, error) {
	scanner := bufio.NewScanner(in)
	procf := New()

	for scanner.Scan() {
		line := scanner.Text()
		result := capture(Regex, line)

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
