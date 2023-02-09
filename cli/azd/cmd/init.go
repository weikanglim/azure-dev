// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/azure/azure-dev/cli/azd/cmd/actions"
	"github.com/azure/azure-dev/cli/azd/internal"
	"github.com/azure/azure-dev/cli/azd/internal/appdetect"
	"github.com/azure/azure-dev/cli/azd/internal/repository"
	"github.com/azure/azure-dev/cli/azd/pkg/account"
	"github.com/azure/azure-dev/cli/azd/pkg/environment"
	"github.com/azure/azure-dev/cli/azd/pkg/environment/azdcontext"
	"github.com/azure/azure-dev/cli/azd/pkg/exec"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/pkg/output"
	"github.com/azure/azure-dev/cli/azd/pkg/output/ux"
	"github.com/azure/azure-dev/cli/azd/pkg/templates"
	"github.com/azure/azure-dev/cli/azd/pkg/tools"
	"github.com/azure/azure-dev/cli/azd/pkg/tools/azcli"
	"github.com/azure/azure-dev/cli/azd/pkg/tools/git"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newInitFlags(cmd *cobra.Command, global *internal.GlobalCommandOptions) *initFlags {
	flags := &initFlags{}
	flags.Bind(cmd.Flags(), global)

	return flags
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new app.",
		//nolint:lll
		Long: `Initialize a new app.

When no template is supplied, you can optionally select an Azure Developer CLI template for cloning. Otherwise, ` + output.WithBackticks("azd init") + ` initializes the current directory and creates resources so that your project is compatible with Azure Developer CLI.

When a template is provided, the sample code is cloned to the current directory.`,
	}

	return cmd
}

type initFlags struct {
	template       templates.Template
	templateBranch string
	subscription   string
	location       string
	global         *internal.GlobalCommandOptions
	*envFlag
}

func (i *initFlags) Bind(local *pflag.FlagSet, global *internal.GlobalCommandOptions) {
	i.bindNonCommon(local, global)
	i.bindCommon(local, global)
}

func (i *initFlags) bindNonCommon(local *pflag.FlagSet, global *internal.GlobalCommandOptions) {
	local.StringVarP(
		&i.template.Name,
		"template",
		"t",
		"",
		//nolint:lll
		"The template to use when you initialize the project. You can use Full URI, <owner>/<repository>, or <repository> if it's part of the azure-samples organization.",
	)
	local.StringVarP(&i.templateBranch, "branch", "b", "", "The template branch to initialize from.")
	local.StringVar(
		&i.subscription,
		"subscription",
		"",
		"Name or ID of an Azure subscription to use for the new environment",
	)
	local.StringVarP(&i.location, "location", "l", "", "Azure location for the new environment")
	i.global = global
}

func (i *initFlags) bindCommon(local *pflag.FlagSet, global *internal.GlobalCommandOptions) {
	i.envFlag = &envFlag{}
	i.envFlag.Bind(local, global)
}

func (i *initFlags) setCommon(envFlag *envFlag) {
	i.envFlag = envFlag
}

type initAction struct {
	azCli           azcli.AzCli
	accountManager  *account.Manager
	console         input.Console
	cmdRun          exec.CommandRunner
	gitCli          git.GitCli
	flags           *initFlags
	repoInitializer *repository.Initializer
}

func newInitAction(
	azCli azcli.AzCli,
	accountManager *account.Manager,
	cmdRun exec.CommandRunner,
	console input.Console,
	gitCli git.GitCli,
	flags *initFlags,
	repoInitializer *repository.Initializer) actions.Action {
	return &initAction{
		azCli:           azCli,
		accountManager:  accountManager,
		console:         console,
		cmdRun:          cmdRun,
		gitCli:          gitCli,
		flags:           flags,
		repoInitializer: repoInitializer,
	}
}

func (i *initAction) Run(ctx context.Context) (*actions.ActionResult, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting cwd: %w", err)
	}

	azdCtx := azdcontext.NewAzdContextWithDirectory(wd)

	if i.flags.templateBranch != "" && i.flags.template.Name == "" {
		return nil, errors.New("template name required when specifying a branch name")
	}

	requiredTools := []tools.ExternalTool{}

	// When using a template, we also require `git`, to acquire the template.
	if i.flags.template.Name != "" {
		requiredTools = append(requiredTools, i.gitCli)
	}

	if err := tools.EnsureInstalled(ctx, requiredTools...); err != nil {
		return nil, err
	}

	if _, err := os.Stat(azdCtx.ProjectPath()); err == nil {
		i.console.Message(ctx, "Already initialized.")
		return nil, nil
	}

	// Command title
	i.console.MessageUxItem(ctx, &ux.MessageTitle{
		Title: "Initializing a new project (azd init)",
	})

	existingApplication, err := i.console.Confirm(ctx, input.ConsoleOptions{
		Message:      "Do you have an existing application?",
		DefaultValue: false,
	})

	if err != nil {
		return nil, err
	}

	if existingApplication {
		appLocationIndex, err := i.console.Select(ctx, input.ConsoleOptions{
			Message: "The Azure Developer CLI can analyze your application and suggest cloud infrastructure. " +
				"Where is your existing application?",
			Options: []string{
				"Under the current directory",
				"On GitHub",
			},
			DefaultValue: "Under the current directory",
		})

		if err != nil {
			return nil, err
		}

		if appLocationIndex == 1 {
			_, err := i.console.Prompt(ctx, input.ConsoleOptions{
				Message: "Please enter a GitHub URL:",
			})

			if err != nil {
				return nil, err
			}
		}

		projects, err := appdetect.Detect(azdCtx.ProjectDirectory())
		useOptions := repository.InfraUseOptions{}
		if err != nil {
			log.Printf("error during detection: %v", err)
		}

		if err == nil && len(projects) > 0 {
			useDetection, err := i.console.Confirm(
				ctx,
				input.ConsoleOptions{
					//nolint:lll
					Message: heredoc.Docf(
						`We found that your existing application is described as follows:
						%s
						Is that right?`,
						describe(projects)),
				})
			if err != nil {
				return nil, err
			}

			if useDetection {
				character := templates.Characteristics{}
				extractCharacteristics(projects, &character, &useOptions)

				i.flags.template, err = templates.MatchOne(ctx, i.console, character)
				if err != nil {
					return nil, err
				}
			}
		}

		if i.flags.template.RepositoryPath == "" {
			useOptions, err = i.searchForTemplate(ctx, useOptions)
			if err != nil && !errors.Is(err, templates.ErrTemplateNotMatched) {
				return nil, err
			}

			if err != nil && errors.Is(err, templates.ErrTemplateNotMatched) {
				showTemplates, err := i.console.Confirm(
					ctx,
					input.ConsoleOptions{
						//nolint:lll
						Message: "Would you like to be shown list of project templates instead?",
					})

				if err != nil || !showTemplates {
					return nil, err
				}

				i.flags.template, err = templates.PromptTemplate(ctx, "Select a project template:", i.console)

				if err != nil {
					return nil, err
				}
			}
		}

		err = i.repoInitializer.InitializeInfra(ctx, azdCtx, i.flags.template.RepositoryPath, "", useOptions)
		if err != nil {
			return nil, fmt.Errorf("initializing infrastructure from template: %w", err)
		}
	} else {
		// Project not initialized and no template specified
		// NOTE: Adding `azure.yaml` to a folder removes the option from selecting a template
		if _, err := os.Stat(azdCtx.ProjectPath()); err != nil && errors.Is(err, os.ErrNotExist) {

			if i.flags.template.Name == "" {
				i.flags.template, err = templates.PromptTemplate(ctx, "Select a project template:", i.console)

				if err != nil {
					return nil, err
				}
			}
		}

		if i.flags.template.Name != "" {
			var templateUrl string

			if i.flags.template.RepositoryPath == "" {
				// using template name directly from command line
				i.flags.template.RepositoryPath = i.flags.template.Name
			}

			// treat names that start with http or git as full URLs and don't change them
			if strings.HasPrefix(i.flags.template.RepositoryPath, "git") ||
				strings.HasPrefix(i.flags.template.RepositoryPath, "http") {
				templateUrl = i.flags.template.RepositoryPath
			} else {
				switch strings.Count(i.flags.template.RepositoryPath, "/") {
				case 0:
					templateUrl = fmt.Sprintf("https://github.com/Azure-Samples/%s", i.flags.template.RepositoryPath)
				case 1:
					templateUrl = fmt.Sprintf("https://github.com/%s", i.flags.template.RepositoryPath)
				default:
					return nil, fmt.Errorf(
						"template '%s' should be either <repository> or <repo>/<repository>", i.flags.template.RepositoryPath)
				}
			}

			err = i.repoInitializer.Initialize(ctx, azdCtx, templateUrl, i.flags.templateBranch)
			if err != nil {
				return nil, fmt.Errorf("init from template repository: %w", err)
			}
		} else {
			err = i.repoInitializer.InitializeEmpty(ctx, azdCtx)
			if err != nil {
				return nil, fmt.Errorf("init empty repository: %w", err)
			}
		}
	}

	envName, err := azdCtx.GetDefaultEnvironmentName()
	if err != nil {
		return nil, fmt.Errorf("retrieving default environment name: %w", err)
	}

	if envName != "" {
		return nil, environment.NewEnvironmentInitError(envName)
	}

	envSpec := environmentSpec{
		environmentName: i.flags.environmentName,
		subscription:    i.flags.subscription,
		location:        i.flags.location,
	}
	env, err := createAndInitEnvironment(ctx, &envSpec, azdCtx, i.console, i.azCli)
	if err != nil {
		return nil, fmt.Errorf("loading environment: %w", err)
	}

	if err := azdCtx.SetDefaultEnvironmentName(envSpec.environmentName); err != nil {
		return nil, fmt.Errorf("saving default environment: %w", err)
	}

	// If the configuration is empty, set default subscription & location
	// This will be the case for first run experience
	if !i.accountManager.HasDefaults() {
		_, err = i.accountManager.SetDefaultSubscription(ctx, env.GetSubscriptionId())
		if err != nil {
			log.Printf("failed setting default subscription. %s\n", err.Error())
		}
		_, err = i.accountManager.SetDefaultLocation(ctx, env.GetSubscriptionId(), env.GetLocation())
		if err != nil {
			log.Printf("failed setting default location. %s\n", err.Error())
		}
	}

	//nolint:lll
	azdTrustNotice := "https://learn.microsoft.com/azure/developer/azure-developer-cli/azd-templates#guidelines-for-using-azd-templates"

	return &actions.ActionResult{
		Message: &actions.ResultMessage{
			Header: "New project initialized!",
			FollowUp: heredoc.Docf(`
			You can view the template code in your directory: %s
			Learn more about running 3rd party code on our DevHub: %s`,
				output.WithLinkFormat("%s", wd),
				output.WithLinkFormat("%s", azdTrustNotice)),
		},
	}, nil
}

func extractCharacteristics(
	projects []appdetect.Project,
	character *templates.Characteristics,
	useOptions *repository.InfraUseOptions) {
	hasOneWeb := false
	for _, project := range projects {
		if project.HasWebUIFramework() {
			hasOneWeb = true
			break
		}
	}

	if hasOneWeb && len(projects) == 1 {
		character.Type = templates.WebApp
	} else if hasOneWeb && len(projects) > 1 {
		character.Type = templates.ApiWeb
	} else {
		character.Type = templates.ApiApp
	}

	for _, project := range projects {
		if project.HasWebUIFramework() {
			useOptions.Projects = append(useOptions.Projects, repository.ProjectSpec{
				Language:  project.Language,
				Host:      "appservice",
				Path:      project.Path,
				HackIsWeb: true,
			})
		} else {
			// HACK: Select first language found.
			if project.Language == "nodejs" {

			}
			useOptions.Language = project.Language
			useOptions.Projects = append(useOptions.Projects, repository.ProjectSpec{
				Language: project.Language,
				Host:     "appservice",
				Path:     project.Path,
			})
		}
	}
}

func (i *initAction) searchForTemplate(
	ctx context.Context,
	useOptions repository.InfraUseOptions) (repository.InfraUseOptions, error) {
	c := &templates.Characteristics{}
	err := templates.PromptToFillCharacteristics(ctx, i.console, c)
	useOptions.Language = c.LanguageTags[0]

	if err != nil {
		return useOptions, err
	}

	i.flags.template, err = templates.MatchOne(ctx, i.console, *c)
	if err != nil {
		return useOptions, err
	}
	return useOptions, err
}

func describe(projects []appdetect.Project) string {
	var b strings.Builder
	for _, p := range projects {
		hasWeb := p.HasWebUIFramework()
		description := "Web API"
		if hasWeb {
			description = "Web App"
		}

		lang := p.Language
		if p.Docker != nil {
			lang += " using containers"
		}

		if hasWeb {
			frameworks := []string{}
			for _, f := range p.Frameworks {
				if f.IsWebUIFramework() {
					frameworks = append(frameworks, f.Display())
				}
			}

			lang = strings.Join(frameworks, ", ")
		}

		b.WriteString(fmt.Sprintf("  - %s (%s) at %s\n", description, lang, p.Path))
	}

	return b.String()
}
