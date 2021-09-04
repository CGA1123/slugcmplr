package slugcmplr

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// StackReplacePattern is used to replace the stack name (e.g. heroku-20)
	// during slugcmplr work.
	StackReplacePattern = "%stack%"

	// StackNumberReplacePattern is used to replace the stack number (e.g. 20)
	// during slugcmplr work.
	StackNumberReplacePattern = "%stack-number%"
)

// Outputter mimics the interface implemented by *cobra.Command to inject
// custom Stdout and Stderr streams, while also allowing control over the
// verbosity of output.
type Outputter interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
	IsVerbose() bool
}

// OutputterFromCmd builds an Outputter based on a *cobra.Command.
func OutputterFromCmd(cmd *cobra.Command, verbose bool) Outputter {
	return &StdOutputter{
		Err:     cmd.ErrOrStderr(),
		Out:     cmd.OutOrStdout(),
		Verbose: verbose,
	}
}

// StdOutputter is an Outputter that will default to os.Stdout and os.Stderr.
type StdOutputter struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

// IsVerbose returnes whether the StdOutputter is in Verbose mode.
func (o *StdOutputter) IsVerbose() bool {
	return o.Verbose
}

// OutOrStdout returns either the configured Out, or os.Stdout if it is nil.
func (o *StdOutputter) OutOrStdout() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}

	return o.Out
}

// ErrOrStderr returns either the configured Err, or os.Stderr if it is nil.
func (o *StdOutputter) ErrOrStderr() io.Writer {
	if o.Err == nil {
		return os.Stdout
	}

	return o.Err
}

// StackImage builds an image name for the given stack.
//
// stack is expected to be in the form `heroku-N` where N is the stack number
// (e.g. 18, 20).
//
// img may container either `%stack%` or `%stack-number%` which will be
// replaced by StackImage with the full stack name or only the number
// accordingly.
func StackImage(img, stack string) string {
	stackNumber := strings.TrimPrefix(stack, "heroku-")

	return strings.ReplaceAll(
		strings.ReplaceAll(img, StackReplacePattern, stack),
		StackNumberReplacePattern,
		stackNumber,
	)
}
