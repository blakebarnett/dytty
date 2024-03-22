package main

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gitlab.com/tozd/go/errors"
)

func TestInvalidEnvName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	cli := CLI{CLIGlobals: CLIGlobals{}}
	NewEnv("invalid", &cli)
}

func TestNewEnv(t *testing.T) {
	name := "development"
	type args struct {
		name string
		cli  CLI
	}

	type want struct {
		want *Environment
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NewEnv": {
			reason: "NewEnv should return a new Env",
			args: args{
				name: name,
			},
			want: want{
				want: &Environment{
					Name: name,
					Paths: EnvPaths{
						Required:       []string{},
						RequiredValues: []string{"values.yaml"},
						Optional:       []string{"resources/*.yaml", "overlays.yaml"},
					},
				}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewEnv(tc.args.name, &tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nEnvironment(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestNewApp(t *testing.T) {
	name := "example"
	kind := "apps"
	env := "development"
	type args struct {
		kind string
		name string
		env  string
		cli  CLI
	}

	type want struct {
		want *App
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NewApp": {
			reason: "NewApp should return a new App with the correct paths and values",
			args: args{
				name: name,
				kind: kind,
				env:  env,
			},
			want: want{
				want: &App{
					Kind: kind,
					BaseApp: BaseApp{
						Name: name,
						Kind: kind,
						Env: Environment{
							Name: env,
							Paths: EnvPaths{
								Required:       []string{},
								RequiredValues: []string{"test-data/envs/development/values.yaml"},
								Optional:       []string{"resources/*.yaml", "overlays.yaml"},
							},
						},
						Paths: Paths{
							Required:       []string{},
							RequiredValues: []string{},
							Optional:       []string{"resources/*.yaml", "overlays.yaml"},
							Templates:      []string{},
						},
					},
					Paths: AppsPaths{
						Required: []string{},
						RequiredValues: []string{
							"test-data/apps/example/base-values.yaml",
							"test-data/apps/example/development/values.yaml",
							"test-data/apps/example/development/image-tag.yaml",
						},
						Optional:  []string{},
						Templates: []string{},
					},
					// This is before the app values are loaded
					Image: AppImage{
						Name:       "",
						Tag:        "0.0.0",
						Repository: "",
						Registry:   "spanio.jfrog.io",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.cli.BasePath = "test-data"
			got := NewApp(tc.args.kind, tc.args.name, tc.args.env, &tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nApp(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestValidatePathsRequiredFilesNotFound(t *testing.T) {
	prefix := "test-data/apps/example"
	required := true
	paths := []string{"reqvalues.yaml"}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	ValidatePaths(prefix, required, paths)
}

func TestAppSetPaths(t *testing.T) {
	env := "development"
	kind := "apps"
	name := "example"
	basePath := "test-data"
	appPrefix := fmt.Sprintf("%s/%s/%s/", basePath, kind, name)
	envPrefix := fmt.Sprintf("%s/envs/%s/", basePath, env)
	appFixture := &App{
		Kind: kind,
		BaseApp: BaseApp{
			Name: name,
			Env: Environment{
				Name: env,
				Paths: EnvPaths{
					Required:       []string{},
					RequiredValues: []string{"values.yaml"},
					Optional:       []string{},
				},
			},
		},
		Paths: AppsPaths{
			Required:       []string{},
			RequiredValues: []string{"base-values.yaml", "development/values.yaml", "development/image-tag.yaml"},
			Optional:       []string{},
			Templates:      []string{},
		},
	}
	type args struct {
		app       *App
		appPrefix string
		envPrefix string
	}

	type want struct {
		want *App
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AppSetPathsAddsCorrectPrefix": {
			reason: "AppSetPaths should add the correct prefix to the paths",
			args: args{
				appPrefix: appPrefix,
				envPrefix: envPrefix,
				app:       appFixture,
			},
			want: want{
				want: &App{
					Kind: kind,
					BaseApp: BaseApp{
						Name: name,
						Env: Environment{
							Name: env,
							Paths: EnvPaths{
								Required:       []string{},
								RequiredValues: []string{envPrefix + "values.yaml"},
								Optional:       []string{},
							},
						},
					},
					Paths: AppsPaths{
						Required:       []string{},
						RequiredValues: []string{appPrefix + "base-values.yaml", appPrefix + env + "/values.yaml", appPrefix + env + "/image-tag.yaml"},
						Optional:       []string{},
						Templates:      []string{},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.app.SetPaths(tc.args.appPrefix, tc.args.envPrefix)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nAppSetPaths(): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}

}

func TestFilesCommandRun(t *testing.T) {
	type args struct {
		kind string
		app  string
		env  string
		cli  *CLI
	}
	type want struct {
		want errors.E
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShouldRunWithoutError": {
			reason: "FilesCommand should run without error",
			args: args{
				kind: "apps",
				app:  "example",
				env:  "development",
				cli:  &CLI{BasePath: "test-data"},
			},
			want: want{want: nil},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &FilesCommand{
				Kind: tc.args.kind,
				App:  tc.args.app,
				Env:  tc.args.env,
			}
			got := c.Run(tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nFilesCommand(): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestValuesCommandRun(t *testing.T) {
	type args struct {
		kind string
		app  string
		env  string
		cli  *CLI
	}
	type want struct {
		want errors.E
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShouldRunWithoutError": {
			reason: "ValuesCommand should run without error",
			args: args{
				kind: "apps",
				app:  "example",
				env:  "development",
				cli:  &CLI{BasePath: "test-data"},
			},
			want: want{want: nil},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &ValuesCommand{
				Kind: tc.args.kind,
				App:  tc.args.app,
				Env:  tc.args.env,
			}
			got := c.Run(tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nValuesCommand(): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestRenderCommandRun(t *testing.T) {
	type args struct {
		kind string
		app  string
		env  string
		cli  *CLI
	}

	fixture := args{
		kind: "apps",
		app:  "example",
		env:  "development",
		cli:  &CLI{BasePath: "test-data"},
	}

	type want struct {
		want errors.E
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShouldRunWithoutError": {
			reason: "RenderCommand should run without error",
			args:   fixture,
			want:   want{want: nil},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &RenderCommand{
				Kind: tc.args.kind,
				App:  tc.args.app,
				Env:  tc.args.env,
			}
			got := c.Run(tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nRenderCommand(): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestYTT(t *testing.T) {
	type args struct {
		app           *App
		inspectValues bool
		inspectFiles  bool
		cli           *CLI
	}

	kind := "apps"
	name := "example"
	env := "development"
	appFixture := NewApp(kind, name, env, &CLI{BasePath: "test-data"})

	type want struct {
		want errors.E
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShouldRunWithoutErrorValues": {
			reason: "Ytt should run without error",
			args: args{
				app:           appFixture,
				inspectValues: true,
				inspectFiles:  false,
				cli:           &CLI{BasePath: "test-data"},
			},
			want: want{want: nil},
		},
		"ShouldRunWithoutErrorInspectFiles": {
			reason: "Ytt should run without error when inspecting files",
			args: args{
				app:           appFixture,
				inspectValues: false,
				inspectFiles:  true,
				cli:           &CLI{BasePath: "test-data"},
			},
			want: want{want: nil},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, got := ytt(tc.args.app, tc.args.inspectValues, tc.args.inspectFiles, tc.args.cli)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nytt(): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
