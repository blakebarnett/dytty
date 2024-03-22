# dytty
A CLI for managing YTT projects and mono-repositories

## Basic usage
Configure your directory structure in a config file (default is `.dytty.yaml`), this tells dytty where to find and in what order to render your templates in. Refer to the (test config file)[./dytty-test-config.yaml] to get started.

Render an application's manifests using `dytty render <kind> <app-name> <environment>`:
`dytty render apps example dev`

Refer to `dytty -h` for more help.

## TODO:
- [ ] Add generic test fixture data
- [ ] Add `new app` command with a skeleton for basics
- [ ] Add `new project` for creating a new repository structure
- [ ] External data sources functionality (terraform outputs, etc.)
- [ ] Potential integration with kapp / guidance on deployment best practices
- [ ] Integration / examples for using kubeconform for linting output
- [ ] Add installation options docs
