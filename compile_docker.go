package slugcmplr

import (
	"context"
	"fmt"
	"os/exec"
)

// CompileDockerCmd wraps up all the information required to run compilation
// within a container.
type CompileDockerCmd struct {
	BuildDir  string
	CacheDir  string
	NetrcPath string
	Image     string
	Stack     string
}

// Execute mounts the build and cache directories, and your netrc file into the
// the container running the provided image, before executing `slugcmplr
// compile`.
func (c *CompileDockerCmd) Execute(ctx context.Context, out Outputter) error {
	dockerRun := exec.CommandContext(ctx, "docker", "run",
		"--volume", fmt.Sprintf("%v:/tmp/build", c.BuildDir),
		"--volume", fmt.Sprintf("%v:/tmp/cache", c.CacheDir),
		"--volume", fmt.Sprintf("%v:/tmp/netrc", c.NetrcPath),
		"--env", "NETRC=/tmp/netrc",
		StackImage(c.Image, c.Stack),
		"compile",
		"--local",
		"--build-dir", "/tmp/build",
		"--cache-dir", "/tmp/cache",
	) // #nosec G204

	dockerRun.Stderr, dockerRun.Stdout = out.ErrOrStderr(), out.OutOrStdout()

	return dockerRun.Run()
}
