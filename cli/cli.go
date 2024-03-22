package cli

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
	"gitlab.com/tozd/go/errors"
	"gitlab.com/tozd/go/zerolog"
)

//nolint:gochecknoglobals
var (
	Version        = ""
	BuildTimestamp = ""
	Revision       = ""
)

const (
	// Exit code 1 is used by Kong, 2 when program panics, 3 when program returns an error.
	errorExitCode = 3
)

type fmtError struct {
	Err error
}

func (e *fmtError) Error() string {
	return fmt.Sprintf("% -+#.1v", errors.Formatter{Error: e.Err}) //nolint:exhaustruct
}

func (e *fmtError) Unwrap() error {
	return e.Err
}

type hasLoggingConfig interface {
	GetLoggingConfig() *zerolog.LoggingConfig
}

func Run(config hasLoggingConfig, vars kong.Vars, run func(*kong.Context) errors.E) {
	// Inside this function, panicking should be set to false before all regular returns from it.
	panicking := true

	parser, err := kong.New(config,
		kong.Description(vars["description"]),
		kong.UsageOnError(),
		kong.Writers(
			os.Stderr,
			os.Stderr,
		),
		kong.Vars{
			"version":                               fmt.Sprintf("version %s (build on %s, git revision %s)", Version, BuildTimestamp, Revision),
			"defaultLoggingConsoleType":             zerolog.DefaultConsoleType,
			"defaultLoggingConsoleLevel":            "warn",
			"defaultLoggingFileLevel":               zerolog.DefaultFileLevel,
			"defaultLoggingMainLevel":               "warn",
			"defaultLoggingContextLevel":            zerolog.DefaultContextLevel,
			"defaultLoggingContextConditionalLevel": zerolog.DefaultContextConditionalLevel,
			"defaultLoggingContextTriggerLevel":     zerolog.DefaultContextTriggerLevel,
		}.CloneWith(vars),
		zerolog.KongLevelTypeMapper,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: % -+#.1v", errors.Formatter{Error: err}) //nolint:exhaustruct
		os.Exit(1)
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		// We use FatalIfErrorf here because it displays usage information. But we use
		// fmtError instead of err so that we format the error and add more details
		// through its Error method, which is called inside FatalIfErrorf.
		parser.FatalIfErrorf(&fmtError{err})
	}

	// Default exist code.
	exitCode := 0

	defer func() {
		if !panicking {
			os.Exit(exitCode)
		}
	}()

	logFile, errE := zerolog.New(config)
	if logFile != nil {
		defer func() {
			err = logFile.Close()
			if err != nil {
				fmt.Fprint(os.Stderr, "error: % -+#.1v", errors.Formatter{Error: err})
			}
		}()
	}

	if errE != nil {
		parser.Fatalf("% -+#.1v", errE)
	}

	// We access main logger through global zerolog logger here, which was set in New.
	// This way we do not have to know anything about the config structure.
	logger := log.Logger

	errE = run(ctx)
	if errE != nil {
		logger.Error().Err(errE).Send()

		exitCode = errorExitCode
	}

	panicking = false
}
