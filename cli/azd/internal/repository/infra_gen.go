package repository

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"unicode"

	"github.com/azure/azure-dev/cli/azd/internal/appdetect"
	"github.com/azure/azure-dev/cli/azd/pkg/environment/azdcontext"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/pkg/osutil"
	"github.com/azure/azure-dev/cli/azd/pkg/output"
	"github.com/azure/azure-dev/cli/azd/pkg/output/ux"
	"github.com/azure/azure-dev/cli/azd/pkg/project"
	"github.com/azure/azure-dev/cli/azd/resources"
	"github.com/otiai10/copy"
)

// A regex that matches against "likely" well-formed database names
var wellFormedDbNameRegex = regexp.MustCompile(`^[a-zA-Z-_0-9]$`)

type DatabasePostgres struct {
	DatabaseUser string
	DatabaseName string
}

type DatabaseCosmos struct {
	DatabaseName string
}

type Parameter struct {
	Name   string
	Value  string
	Type   string
	Secret bool
}

type InfraSpec struct {
	Name       string
	Parameters []Parameter
	Services   []ServiceSpec

	// Databases to create
	DbPostgres *DatabasePostgres
	DbCosmos   *DatabaseCosmos
}

type Frontend struct {
	Framework appdetect.Framework
	Backends  []ServiceSpec
}

type Backend struct {
	Frontends []ServiceSpec
}

type EntryKind string

const (
	EntryKindUnknown   EntryKind = ""
	EntryKindDetection EntryKind = "detection"
	EntryKindManual    EntryKind = "manual"
	EntryKindModified  EntryKind = "modified"
)

type ServiceMetadata struct {
	Entry       EntryKind
	DisplayName string
}

type ServiceSpec struct {
	Name string
	Port int
	Path string

	Host     project.ServiceTargetKind
	Language project.ServiceLanguageKind
	Metadata ServiceMetadata

	// Front-end properties.
	Frontend *Frontend

	// Back-end properties
	Backend *Backend

	// Connection to a database. Only one should be set.
	DbPostgres *DatabasePostgres
	DbCosmos   *DatabaseCosmos
}

func supportedLanguages() []appdetect.ProjectType {
	return []appdetect.ProjectType{
		appdetect.DotNet,
		appdetect.Java,
		appdetect.JavaScript,
		appdetect.TypeScript,
		appdetect.Python,
	}
}

func mapLanguage(l appdetect.ProjectType) project.ServiceLanguageKind {
	switch l {
	case appdetect.Python:
		return project.ServiceLanguagePython
	case appdetect.DotNet:
		return project.ServiceLanguageDotNet
	case appdetect.JavaScript:
		return project.ServiceLanguageJavaScript
	case appdetect.TypeScript:
		return project.ServiceLanguageTypeScript
	case appdetect.Java:
		return project.ServiceLanguageJava
	default:
		return ""
	}
}

func supportedFrameworks() []appdetect.Framework {
	return []appdetect.Framework{
		appdetect.Angular,
		appdetect.JQuery,
		appdetect.VueJs,
		appdetect.React,
	}
}

type DatabaseKind string

const (
	DbPostgre     DatabaseKind = "postgres"
	DbCosmosMongo DatabaseKind = "cosmos-mongo"
)

func mapDatabase(d appdetect.Framework) DatabaseKind {
	switch d {
	case appdetect.DbMongo:
		return DbCosmosMongo
	case appdetect.DbPostgres:
		return DbPostgre
	default:
		return ""
	}
}

func supportedDatabases() []DatabaseKind {
	return []DatabaseKind{
		DbPostgre,
		DbCosmosMongo,
	}
}

func (f DatabaseKind) Display() string {
	switch f {
	case DbPostgre:
		return "PostgreSQL"
	case DbCosmosMongo:
		return "MongoDB"
	}

	return ""
}

func detectionToSpec(root string, projects []appdetect.Project) (InfraSpec, error) {
	spec := InfraSpec{
		Name: filepath.Base(root),
	}

	for _, prj := range projects {
		serviceSpec := ServiceSpec{}
		rel, err := filepath.Rel(root, prj.Path)
		if err != nil {
			return spec, err
		}

		serviceSpec.Name = filepath.Base(rel)
		serviceSpec.Host = project.ContainerAppTarget
		serviceSpec.Path = rel
		serviceSpec.Metadata.Entry = EntryKindDetection
		serviceSpec.Metadata.DisplayName = prj.Language.Display()

		switch prj.Language {
		case appdetect.Python:
			serviceSpec.Language = project.ServiceLanguagePython
		case appdetect.DotNet:
			serviceSpec.Language = project.ServiceLanguageDotNet
		case appdetect.JavaScript:
			serviceSpec.Language = project.ServiceLanguageJavaScript
		case appdetect.TypeScript:
			serviceSpec.Language = project.ServiceLanguageTypeScript
		case appdetect.Java:
			serviceSpec.Language = project.ServiceLanguageJava
		default:
			panic(fmt.Sprintf("unhandled language: %s", string(prj.Language)))
		}

		for _, framework := range prj.Frameworks {
			if framework.IsDatabaseDriver() {
				kind := mapDatabase(framework)
				if kind == "" {
					continue
				}

				switch kind {
				case DbPostgre:
					if spec.DbPostgres == nil {
						spec.DbPostgres = &DatabasePostgres{}
					}
					serviceSpec.DbPostgres = spec.DbPostgres
				case DbCosmosMongo:
					if spec.DbCosmos == nil {
						spec.DbCosmos = &DatabaseCosmos{}
					}
					serviceSpec.DbCosmos = spec.DbCosmos
				}
			}

			if framework.IsWebUIFramework() {
				serviceSpec.Metadata.DisplayName = framework.Display()
				serviceSpec.Frontend = &Frontend{}
			}
		}

		spec.Services = append(spec.Services, serviceSpec)
	}

	return spec, nil
}

func (i *Initializer) InitializeInfra(
	ctx context.Context,
	azdCtx *azdcontext.AzdContext) error {
	wd := azdCtx.ProjectDirectory()
	title := "Scanning app code in " + output.WithHighLightFormat(wd)
	i.console.ShowSpinner(ctx, title, input.Step)
	projects, err := appdetect.Detect(wd)
	i.console.StopSpinner(ctx, title, input.GetStepResultFormat(err))

	if err != nil {
		return err
	}

	spec, err := detectionToSpec(wd, projects)
	if err != nil {
		return err
	}

	firstConfirmation := true

confirmDetection:
	for {
		if firstConfirmation {
			i.console.MessageUxItem(ctx, &ux.DoneMessage{Message: "Generating recommended Azure services"})
			i.console.Message(ctx, "\n"+output.WithBold("App detection summary:")+"\n")
		} else {
			i.console.MessageUxItem(ctx, &ux.DoneMessage{Message: "Revising app detection summary"})
			i.console.Message(ctx, "\n"+output.WithBold("Revised app detection summary:")+"\n")
		}
		firstConfirmation = false

		for _, svc := range spec.Services {
			status := ""
			switch svc.Metadata.Entry {
			case EntryKindManual:
				status = " [Added]"
			case EntryKindModified:
				status = " [Updated]"
			}

			i.console.Message(ctx, "  "+output.WithBold(svc.Metadata.DisplayName)+status)
			relDisplay := svc.Path
			if relDisplay == "" {
				relDisplay = "."
			} else {
				relDisplay = "." + string(filepath.Separator) + relDisplay
			}
			i.console.Message(ctx, "  "+"Detected in: "+output.WithHighLightFormat(relDisplay))
			i.console.Message(ctx, "  "+"Recommended: "+"Azure Container Apps")
			i.console.Message(ctx, "")
		}

		if spec.DbCosmos != nil {
			i.console.Message(ctx, "  "+output.WithBold("MongoDB"))
			i.console.Message(ctx, "  "+"Recommended: CosmosDB API for MongoDB")
			i.console.Message(ctx, "")
		}

		if spec.DbPostgres != nil {
			i.console.Message(ctx, "  "+output.WithBold("PostgreSQL"))
			i.console.Message(ctx, "  "+"Recommended: Azure Database for PostgreSQL flexible server")
			i.console.Message(ctx, "")
		}

		i.console.Message(ctx,
			"azd will generate the files necessary to host your app on Azure using the recommended services.")

		continueOption, err := i.console.Select(ctx, input.ConsoleOptions{
			Message: "Select an option",
			Options: []string{
				"Configure my app to run on Azure using the recommended services",
				"Add a language or database azd failed to detect",
				"Modify or remove a detected language or database",
			},
		})
		if err != nil {
			return err
		}

		switch continueOption {
		case 0:
			break confirmDetection
		case 1:
			languages := supportedLanguages()
			frameworks := supportedFrameworks()
			databases := supportedDatabases()
			selections := make([]string, 0, len(languages)+len(databases))
			entries := make([]any, 0, len(languages)+len(databases))

			for _, lang := range languages {
				selections = append(selections, fmt.Sprintf("%s\t%s", lang.Display(), "[Language]"))
				entries = append(entries, lang)
			}

			for _, framework := range frameworks {
				selections = append(selections, fmt.Sprintf("%s\t%s", framework.Display(), "[Framework]"))
				entries = append(entries, framework)
			}

			for _, db := range databases {
				selections = append(selections, fmt.Sprintf("%s\t%s", db.Display(), "[Database]"))
				entries = append(entries, db)
			}

			tabbed := strings.Builder{}
			tabW := tabwriter.NewWriter(&tabbed, 0, 0, 3, ' ', 0)
			_, err = tabW.Write([]byte(strings.Join(selections, "\n")))
			if err != nil {
				return err
			}
			err = tabW.Flush()
			if err != nil {
				return err
			}

			selections = strings.Split(tabbed.String(), "\n")

			entIdx, err := i.console.Select(ctx, input.ConsoleOptions{
				Message: "Select a language or database to add",
				Options: selections,
			})
			if err != nil {
				return err
			}

			s := ServiceSpec{}
			switch entries[entIdx].(type) {
			case appdetect.ProjectType:
				detectLanguage := entries[entIdx].(appdetect.ProjectType)
				language := mapLanguage(detectLanguage)
				if language == "" {
					log.Panicf("unhandled language: %s", string(detectLanguage))
				}

				s.Language = language
				s.Metadata.DisplayName = detectLanguage.Display()
			case appdetect.Framework:
				framework := entries[entIdx].(appdetect.Framework)
				if framework.IsWebUIFramework() {
					s.Language = project.ServiceLanguageJavaScript
					s.Frontend = &Frontend{}
				}
				s.Metadata.DisplayName = framework.Display()
			case DatabaseKind:
				db := entries[entIdx].(DatabaseKind)
				switch db {
				case DbCosmosMongo:
					if spec.DbCosmos != nil {
						spec.DbCosmos = &DatabaseCosmos{}
					}
				case DbPostgre:
					if spec.DbPostgres != nil {
						spec.DbPostgres = &DatabasePostgres{}
					}
				default:
					log.Panicf("unhandled database: %s", string(db))
				}
			}

			for {
				path, err := i.console.Prompt(ctx, input.ConsoleOptions{
					Message: fmt.Sprintf("Enter file path of the directory that uses '%s'", s.Metadata.DisplayName),
					Options: selections,
					Suggest: func(input string) (completions []string) {
						matches, _ := filepath.Glob(input + "*")
						for _, match := range matches {
							if fs, err := os.Stat(match); err == nil && fs.IsDir() {
								completions = append(completions, match)
							}
						}

						return completions
					},
				})
				if err != nil {
					return err
				}

				fs, err := os.Stat(path)
				if errors.Is(err, os.ErrNotExist) || fs != nil && !fs.IsDir() {
					i.console.Message(ctx, fmt.Sprintf("'%s' is not a valid directory", path))
					continue
				}

				if err != nil {
					return err
				}

				path, err = filepath.Abs(path)
				if err != nil {
					return err
				}

				for idx, svc := range spec.Services {
					if svc.Path == path {
						i.console.Message(
							ctx,
							fmt.Sprintf(
								"\nazd previously detected '%s' at %s.\n", svc.Metadata.DisplayName, svc.Path))

						confirm, err := i.console.Confirm(ctx, input.ConsoleOptions{
							Message: fmt.Sprintf(
								"Do you want to change the detected service to '%s'", s.Metadata.DisplayName),
						})
						if err != nil {
							return err
						}
						if confirm {
							spec.Services[idx].Language = svc.Language
							spec.Services[idx].Frontend = svc.Frontend
							spec.Services[idx].Metadata.DisplayName = svc.Metadata.DisplayName
							spec.Services[idx].Metadata.Entry = EntryKindModified
						}

						continue confirmDetection
					}
				}

				s.Path = filepath.Clean(path)
				s.Metadata.Entry = EntryKindManual
				spec.Services = append(spec.Services, s)
				break
			}
		}
	}

	detectedDbs := make(map[DatabaseKind]struct{})
	if spec.DbPostgres != nil {
		detectedDbs[DbPostgre] = struct{}{}
	}
	if spec.DbCosmos != nil {
		detectedDbs[DbCosmosMongo] = struct{}{}
	}

	for database := range detectedDbs {
	dbPrompt:
		for {
			dbName, err := i.console.Prompt(ctx, input.ConsoleOptions{
				Message: "Input a name for the app database",
			})
			if err != nil {
				return err
			}

			if dbName == "" {
				continue dbPrompt
			}

			if strings.ContainsAny(dbName, " ") {
				confirm, err := i.console.Confirm(ctx, input.ConsoleOptions{
					Message: "Database name contains whitespace. " +
						"This may not be allowed by the database server. Continue?",
				})
				if err != nil {
					return err
				}

				if confirm {
					break dbPrompt
				} else {
					continue dbPrompt
				}
			}

			if !wellFormedDbNameRegex.MatchString(dbName) {
				confirm, err := i.console.Confirm(ctx, input.ConsoleOptions{
					Message: "Database name contains special characters. " +
						"This may not be allowed by the database server. Continue?",
				})
				if err != nil {
					return err
				}

				if !confirm {
					continue dbPrompt
				}
			}

			switch database {
			case DbCosmosMongo:
				spec.DbCosmos.DatabaseName = dbName
				break dbPrompt
			case DbPostgre:
				spec.DbPostgres.DatabaseName = dbName
				spec.Parameters = append(spec.Parameters,
					Parameter{
						Name:   "sqlAdminPassword",
						Value:  "$(secretOrRandomPassword)",
						Type:   "string",
						Secret: true,
					},
					Parameter{
						Name:   "appUserPassword",
						Value:  "$(secretOrRandomPassword)",
						Type:   "string",
						Secret: true,
					})
				break dbPrompt
			}
		}
	}

	backends := []ServiceSpec{}
	frontends := []ServiceSpec{}
	for _, svc := range spec.Services {
		var port int
		for {
			val, err := i.console.Prompt(ctx, input.ConsoleOptions{
				Message: "What port does '" + svc.Name + "' listen on? (0 means no exposed ports)",
			})
			if err != nil {
				return err
			}

			port, err = strconv.Atoi(val)
			if err == nil {
				break
			}
			i.console.Message(ctx, "Must be an integer. Try again or press Ctrl+C to cancel")
		}

		svc.Port = port
		if svc.Frontend == nil && svc.Port > 0 {
			backends = append(backends, svc)
			svc.Backend = &Backend{}
		} else {
			frontends = append(frontends, svc)
		}
	}

	// Link services together
	for _, service := range spec.Services {
		if service.Frontend != nil {
			service.Frontend.Backends = backends
		}

		if service.Backend != nil {
			service.Backend.Frontends = frontends
		}

		spec.Parameters = append(spec.Parameters, Parameter{
			Name:  bicepName(service.Name) + "Exists",
			Value: fmt.Sprintf("${SERVICE_%s_RESOURCE_EXISTS=false}", strings.ToUpper(service.Name)),
			Type:  "bool",
		})
	}

	confirm, err := i.console.Select(ctx, input.ConsoleOptions{
		Message: "Do you want to continue?",
		Options: []string{
			"Yes - Generate files to host my app on Azure using the recommended services",
			"No - Modify detected languages or databases",
		},
	})
	if err != nil {
		return err
	}

	if confirm == 1 {
		// modify
		panic("modify unimplemented")
	}

	generateProject := func() error {
		title := "Generating " + output.WithBold(azdcontext.ProjectFileName) +
			" in " + output.WithHighLightFormat(azdCtx.ProjectDirectory())
		i.console.ShowSpinner(ctx, title, input.Step)
		defer i.console.StopSpinner(ctx, title, input.GetStepResultFormat(err))
		config, err := DetectionToConfig(wd, projects)
		if err != nil {
			return fmt.Errorf("converting config: %w", err)
		}
		err = project.Save(
			context.Background(),
			&config,
			filepath.Join(wd, azdcontext.ProjectFileName))
		if err != nil {
			return fmt.Errorf("generating azure.yaml: %w", err)
		}
		return nil
	}

	err = generateProject()
	if err != nil {
		return err
	}

	target := filepath.Join(azdCtx.ProjectDirectory(), "infra")
	title = "Generating Infrastructure as Code files in " + output.WithHighLightFormat(target)
	i.console.ShowSpinner(ctx, title, input.Step)
	defer i.console.StopSpinner(ctx, title, input.GetStepResultFormat(err))

	staging, err := os.MkdirTemp("", "azd-infra")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}

	defer func() { _ = os.RemoveAll(staging) }()

	err = copyFS(resources.ScaffoldBase, "scaffold/base", staging)
	if err != nil {
		return fmt.Errorf("copying to staging: %w", err)
	}

	stagingApp := filepath.Join(staging, "app")
	if err := os.MkdirAll(stagingApp, osutil.PermissionDirectory); err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"bicepName": bicepName,
		"upper":     strings.ToUpper,
	}

	root := "scaffold/templates"
	t, err := template.New("templates").
		Option("missingkey=error").
		Funcs(funcMap).
		ParseFS(resources.ScaffoldTemplates,
			path.Join(root, "*.bicept"),
			path.Join(root, "*.jsont"))
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	if spec.DbCosmos != nil {
		err = execute(t, "db-cosmos.bicep", spec.DbCosmos, filepath.Join(stagingApp, "db-cosmos.bicep"))
		if err != nil {
			return err
		}
	}

	if spec.DbPostgres != nil {
		err = execute(t, "db-postgre.bicep", spec.DbPostgres, filepath.Join(stagingApp, "db-postgre.bicep"))
		if err != nil {
			return err
		}
	}

	for _, svc := range spec.Services {
		err = execute(t, "host-containerapp.bicep", svc, filepath.Join(stagingApp, svc.Name+".bicep"))
		if err != nil {
			return err
		}
	}

	err = execute(t, "main.bicep", spec, filepath.Join(staging, "main.bicep"))
	if err != nil {
		return err
	}

	err = execute(t, "main.parameters.json", spec, filepath.Join(staging, "main.parameters.json"))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, osutil.PermissionDirectory); err != nil {
		return err
	}

	if err := copy.Copy(staging, target); err != nil {
		return fmt.Errorf("copying contents from temp staging directory: %w", err)
	}

	return nil
}

func execute(t *template.Template, name string, data any, writePath string) error {
	buf := bytes.NewBufferString("")
	err := t.ExecuteTemplate(buf, name, data)
	if err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	err = os.WriteFile(writePath, buf.Bytes(), osutil.PermissionFile)
	if err != nil {
		return fmt.Errorf("writing service file: %w", err)
	}
	return nil
}

func bicepName(name string) string {
	sb := strings.Builder{}
	separatorStart := -1
	for pos, char := range name {
		switch char {
		case '-', '_':
			separatorStart = pos
		default:
			if separatorStart != -1 {
				char = unicode.ToUpper(char)
			}
			separatorStart = -1

			if _, err := sb.WriteRune(char); err != nil {
				panic(err)
			}
		}
	}

	return sb.String()
}

func copyFS(embedFs embed.FS, root string, target string) error {
	return fs.WalkDir(embedFs, root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, name[len(root):])

		if d.IsDir() {
			return os.MkdirAll(targetPath, osutil.PermissionDirectory)
		}

		contents, err := fs.ReadFile(embedFs, name)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		return os.WriteFile(targetPath, contents, osutil.PermissionFile)
	})
}
