package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	yttcmd "carvel.dev/ytt/pkg/cmd/template"
	yttui "carvel.dev/ytt/pkg/cmd/ui"
	yttfiles "carvel.dev/ytt/pkg/files"
	"github.com/alecthomas/kong"
	"gitlab.com/tozd/go/errors"
	"gitlab.com/tozd/go/zerolog"
	yaml "gopkg.in/yaml.v3"

	"github.com/blakebarnett/dytty/cli"
)

type CLIGlobals struct {
	zerolog.LoggingConfig `yaml:",inline"`

	Version kong.VersionFlag `help:"Show program's version and exit."                                              short:"V" yaml:"-"`
	Config  cli.ConfigFlag   `help:"Load configuration from a JSON or YAML file. (default: .dytty.yaml)" name:"config" placeholder:"PATH" short:"c" yaml:"-" default:".dytty.yaml"`
}

type CLI struct {
	CLIGlobals `yaml:"cliglobals"`

	Global struct {
		Paths struct {
			Required       []string `name:"required" yaml:"required"`
			RequiredValues []string `name:"required-values" yaml:"requiredValues"`
			Optional       []string `name:"optional" yaml:"optional"`
		} `cmd:"" yaml:"paths" hidden:"true"`
	} `cmd:"" yaml:"global" hidden:"true"`
	Kinds struct {
		Apps struct {
			Paths struct {
				Required       []string `name:"required" yaml:"required"`
				RequiredValues []string `name:"required-values" yaml:"requiredValues"`
				Optional       []string `name:"optional" yaml:"optional"`
				Templates      []string `name:"templates" yaml:"templates"`
			} `cmd:"" yaml:"paths" hidden:"true"`
		} `cmd:"" yaml:"apps" hidden:"true"`
	} `cmd:"" yaml:"kinds" hidden:"true"`
	Environments map[string]Environment `name:"environments" yaml:"environments" hidden:"true"`
	BasePath     string                 `help:"Base path for the application." name:"base-path" placeholder:"PATH" short:"b" yaml:"basePath"`
	ImageTag     string                 `help:"The image tag to use for the application." name:"image-tag" placeholder:"TAG" short:"t" yaml:"imageTag"`
	Render       RenderCommand          `cmd:"" help:"Render manifests for an application." yaml:"render"`
	Values       ValuesCommand          `cmd:"" help:"Render data values for an application." yaml:"values"`
	Files        FilesCommand           `cmd:"" help:"Inspect all files involved for rendering an application." yaml:"files"`
}

type BaseApp struct {
	Name  string
	Env   Environment
	Kind  string
	Paths Paths
}

type App struct {
	BaseApp
	Kind  string `default:"apps" yaml:"kind"`
	Paths AppsPaths
	Image AppImage
}

type Serverless struct {
	BaseApp
	Kind  string `default:"lambda" yaml:"kind"`
	Paths ServerlessPaths
}

type Infra struct {
	BaseApp
	Kind  string `default:"infra" yaml:"kind"`
	Paths InfraPaths
}

type Paths struct {
	Required       []string `name:"required" yaml:"required"`
	RequiredValues []string `name:"required-values" yaml:"requiredValues"`
	Optional       []string `name:"optional" yaml:"optional"`
	Templates      []string `default:"[]" yaml:"templates"`
}

type EnvPaths struct {
	Required       []string `name:"required" yaml:"required"`
	RequiredValues []string `name:"required-values" yaml:"requiredValues"`
	Optional       []string `name:"optional" yaml:"optional"`
}

type Environment struct {
	Name  string
	Paths EnvPaths
}

type AppImage struct {
	Name       string `default:"" yaml:"name"`
	Tag        string `default:"0.0.0"`
	Repository string `default:"" yaml:"repository"`
	Registry   string `default:"spanio.jfrog.io" yaml:"registry"`
}

type AppsPaths struct {
	Required       []string `name:"required" yaml:"required"`
	RequiredValues []string `name:"required-values" yaml:"requiredValues"`
	Optional       []string `name:"optional" yaml:"optional"`
	Templates      []string `default:"[]" yaml:"templates"`
}

type InfraPaths struct {
}

type ServerlessPaths struct {
}

type RenderCommand struct {
	Kind string `arg:"" help:"The application kind." name:"kind" enum:"apps,lambda,infra" yaml:"kind"`
	App  string `arg:"" help:"The application name." name:"app" yaml:"app"`
	Env  string `arg:"" help:"The environment name." name:"env" enum:"dev,development,int,integration,prd,prod,production" yaml:"env"`
}

type ValuesCommand struct {
	Kind string `arg:"" help:"The application kind." name:"kind" enum:"apps,lambda,infra" yaml:"kind"`
	App  string `arg:"" help:"The application name." name:"app" yaml:"app"`
	Env  string `arg:"" help:"The environment name." name:"env" enum:"dev,development,int,integration,prd,prod,production" yaml:"env"`
}

type FilesCommand struct {
	Kind string `arg:"" help:"The application kind." name:"kind" enum:"apps,lambda,infra" yaml:"kind"`
	App  string `arg:"" help:"The application name." name:"app" yaml:"app"`
	Env  string `arg:"" help:"The environment name." name:"env" enum:"dev,development,int,integration,prd,prod,production" yaml:"env"`
}

func NewEnv(name string, cli *CLI) *Environment {
	env := &Environment{}
	logger := cli.GetLoggingConfig().Logger
	logger.Debug().Msgf("Creating new env: %s", name)

	nn, err := NormalizeEnvName(name)
	if err != nil {
		panic(err)
	}

	env.Name = nn
	logger.Debug().Msgf("Env: %v", env)

	return env
}

func NewApp(kind string, name string, env string, cli *CLI) *App {
	logger := cli.GetLoggingConfig().Logger
	logger.Debug().Msgf("Creating new app: %s, %s, %s", kind, name, env)
	app := &App{
		BaseApp: BaseApp{
			Name: name,
			Env:  *NewEnv(env, cli),
			Kind: kind,
		},
	}

	if cli.ImageTag != "" {
		app.Image.Tag = cli.ImageTag
	}

	app.SetPaths(cli)

	buf := io.Writer(bytes.NewBuffer([]byte{}))

	logger.Debug().Msgf("App: %s", yaml.NewEncoder(buf).Encode(app))

	return app
}

func NormalizeEnvName(e string) (string, error) {
	err := error(nil)

	switch e {
	case "dev", "development":
		return "development", nil
	case "int", "integration":
		return "integration", nil
	case "prd", "prod", "production":
		return "production", nil
	default:
		err = fmt.Errorf("invalid environment name: %s", e)
	}

	return "", err
}

func ValidatePaths(required bool, paths []string) []string {
	files := []string{}

	for _, path := range paths {
		if path == "" {
			continue
		}

		matches, err := filepath.Glob(path)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			if required {
				panic(err)
			}
		} else if matches == nil {
			if required {
				panic(fmt.Errorf("no files found for required path: %s", (path)))
			}
		}

		files = append(files, matches...)
	}

	return files
}

func (app *App) renderPathTemplates(templates []string) []string {
	results := []string{}

	for _, ts := range templates {
		t, err := template.New("app").Parse(ts)
		if err != nil {
			panic(err)
		}

		var buf bytes.Buffer

		err = t.Execute(&buf, app)
		if err != nil {
			panic(err)
		}

		results = append(results, buf.String())
	}

	return results
}

func (app *App) SetPaths(cli *CLI) *App {
	// The first two will panic if the paths do not exist as they set required to true
	app.Paths.Required = ValidatePaths(true, app.renderPathTemplates(cli.Kinds.Apps.Paths.Required))
	app.Paths.RequiredValues = ValidatePaths(true, app.renderPathTemplates(cli.Kinds.Apps.Paths.RequiredValues))
	app.Paths.Optional = ValidatePaths(false, app.renderPathTemplates(cli.Kinds.Apps.Paths.Optional))

	app.Env.Paths.Required = ValidatePaths(true, app.renderPathTemplates(cli.Environments[app.Env.Name].Paths.Required))
	app.Env.Paths.RequiredValues = ValidatePaths(true, app.renderPathTemplates(cli.Environments[app.Env.Name].Paths.RequiredValues))

	return app
}

func (c *FilesCommand) Run(cli *CLI) errors.E {
	logger := cli.GetLoggingConfig().Logger
	logger.Info().Msgf("Files for kind: %s, app: %s, env: %s", c.Kind, c.App, c.Env)
	app := NewApp(c.Kind, c.App, c.Env, cli)

	data, err := ytt(app, false, true, cli)
	if err != nil {
		panic(err)
	}

	results := []string{}

	err = yaml.Unmarshal([]byte(data), &results)
	if err != nil {
		return errors.Errorf("error parsing yaml: %v", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", strings.Join(results, "\n"))

	return nil
}

func ParseValues(app *App, cli *CLI) (map[string]any, error) {
	logger := cli.GetLoggingConfig().Logger
	results := make(map[string]any)

	data, err := ytt(app, true, false, cli)
	if err != nil {
		return nil, errors.Errorf("ParseValues() error: %v", err)
	}

	err = yaml.Unmarshal(data, &results)
	if err != nil {
		log.Fatalf("ParseValues(): %v", err)
	}

	logger.Debug().Msgf("RESULTS ParseValues: %s", results)

	return results, nil
}

func (c *ValuesCommand) Run(cli *CLI) errors.E {
	logger := cli.GetLoggingConfig().Logger
	logger.Info().Msgf("Values for kind: %s, app: %s, env: %s", c.Kind, c.App, c.Env)
	app := NewApp(c.Kind, c.App, c.Env, cli)

	results, err := ParseValues(app, cli)
	if err != nil {
		panic(err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", results)

	return nil
}

func (c *RenderCommand) Run(cli *CLI) errors.E { //nolint:unparam
	logger := cli.GetLoggingConfig().Logger
	logger.Info().Msgf("Rendering for kind: %s, app: %s, env: %s", c.Kind, c.App, c.Env)
	app := NewApp(c.Kind, c.App, c.Env, cli)

	logger.Info().Msgf("Required paths: %s", app.Paths.Required)
	logger.Info().Msgf("Required Data Values paths: %s", app.Paths.RequiredValues)
	logger.Info().Msgf("Env Required paths: %s", app.Env.Paths.Required)
	logger.Info().Msgf("Env Data Values paths: %s", app.Env.Paths.RequiredValues)
	logger.Info().Msgf("Optional paths: %s", app.Paths.Optional)

	// Render the data values
	data, err := ytt(app, true, false, cli)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, &app.Paths)
	if err != nil {
		logger.Error().Msgf("Error unmarshalling data: %s", err)
		panic(err)
	}

	// Validate the templates exist also
	templates := []string{}
	for _, t := range app.Paths.Templates {
		templates = append(templates, cli.BasePath+"/templates/"+t)
	}

	app.Paths.Templates = ValidatePaths(true, templates)
	logger.Info().Msgf("Template paths: %s", app.Paths.Templates)

	results, err := ytt(app, false, false, cli)
	if err != nil {
		panic(err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", results)

	return nil
}

func ytt(app *App, inspectValues bool, inspectFiles bool, cli *CLI) ([]byte, error) {
	opts := *yttcmd.NewOptions()
	opts.InspectFiles = inspectFiles
	opts.DataValuesFlags.Inspect = inspectValues
	ui := yttui.NewCustomWriterTTY(false, os.Stdout, os.Stderr)
	paths := []string{cli.BasePath + "/global"}
	paths = append(paths, app.Env.Paths.RequiredValues...)
	paths = append(paths, app.Paths.RequiredValues...)
	paths = append(paths, app.Env.Paths.Required...)
	paths = append(paths, app.Paths.Required...)
	paths = append(paths, app.Paths.Optional...)
	paths = append(paths, app.Paths.Templates...)

	files, err := addFiles(opts, paths...)
	if err != nil {
		return []byte{}, err
	}

	// Evaluate the template given the configured data values.
	input := yttcmd.Input{Files: files}

	tag := "app.image.tag=" + app.Image.Tag
	opts.DataValuesFlags.KVsFromStrings = append(opts.DataValuesFlags.KVsFromStrings, tag)

	output := opts.RunWithFiles(input, ui)
	if output.Err != nil {
		return []byte{}, output.Err
	}

	// output.DocSet contains the full set of resulting YAML documents, in order.
	bs, err := output.DocSet.AsBytes()
	if err != nil {
		return []byte{}, err
	}

	return bs, nil
}

func addFiles(opts yttcmd.Options, yttpaths ...string) ([]*yttfiles.File, error) {
	var files []*yttfiles.File

	files, err := yttfiles.NewSortedFilesFromPaths(yttpaths, *opts.RegularFilesSourceOpts.SymlinkAllowOpts)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func main() {
	var c CLI

	cli.Run(&c, nil, func(ctx *kong.Context) errors.E {
		logger := c.GetLoggingConfig().Logger

		path, err := os.Getwd()
		if err != nil {
			return errors.Errorf("error getting current working directory: %v", err)
		}

		fqpath, err := filepath.Abs(path)
		if err != nil {
			return errors.Errorf("error getting absolute path: %v", err)
		}

		logger.Warn().Msgf("CLI Config loaded from: %s/%s", fqpath, c.Config)
		//logger.Debug().Msgf("CLI Config: %s", yaml.NewEncoder(os.Stderr).Encode(&c))
		return errors.WithStack(ctx.Run(&c))
	})
}
