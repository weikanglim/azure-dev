package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azure/azure-dev/cli/azd/internal/appdetect"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/test/ostest"
	"github.com/stretchr/testify/require"
)

func Test_detectConfirm_confirm(t *testing.T) {
	dir := t.TempDir()
	dotNetDir := filepath.Join(dir, "dotnet-dir")
	err := os.MkdirAll(dotNetDir, 0700)
	require.NoError(t, err)
	javaDir := filepath.Join(dir, "java-dir")
	err = os.MkdirAll(javaDir, 0700)
	require.NoError(t, err)
	ostest.Chdir(t, dir)

	tests := []struct {
		name         string
		detection    []appdetect.Project
		interactions []string
		want         []appdetect.Project
	}{
		{
			name: "confirm single",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
			interactions: []string{
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
		},
		{
			name: "add a language",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
			interactions: []string{
				"Add an undetected service",
				fmt.Sprintf("%s\t%s", appdetect.Java.Display(), "[Language]"),
				"java-dir",
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
				{
					Language:      appdetect.Java,
					Path:          javaDir,
					DetectionRule: string(EntryKindManual),
				},
			},
		},
		{
			name: "add a framework",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
			interactions: []string{
				"Add an undetected service",
				fmt.Sprintf("%s\t%s", appdetect.JsReact.Display(), "[Framework]"),
				"java-dir",
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
				{
					Language:      appdetect.JavaScript,
					Path:          javaDir,
					Dependencies:  []appdetect.Dependency{appdetect.JsReact},
					DetectionRule: string(EntryKindManual),
				},
			},
		},
		{
			name: "remove a language",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
			interactions: []string{
				"Remove a detected service",
				fmt.Sprintf("%s in %s", appdetect.DotNet.Display(), "dotnet-dir"),
				"y",
				"Add an undetected service",
				fmt.Sprintf("%s\t%s", appdetect.Java.Display(), "[Language]"),
				"java-dir",
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language:      appdetect.Java,
					Path:          javaDir,
					DetectionRule: string(EntryKindManual),
				},
			},
		},
		{
			name: "add a database",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
				},
			},
			interactions: []string{
				"Add an undetected service",
				fmt.Sprintf("%s\t%s", appdetect.DbPostgres.Display(), "[Database]"),
				fmt.Sprintf("%s in %s", appdetect.DotNet.Display(), "dotnet-dir"),
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
					DatabaseDeps: []appdetect.DatabaseDep{
						appdetect.DbPostgres,
					},
					DetectionRule: string(EntryKindModified),
				},
			},
		},
		{
			name: "remove a database",
			detection: []appdetect.Project{
				{
					Language: appdetect.DotNet,
					Path:     dotNetDir,
					DatabaseDeps: []appdetect.DatabaseDep{
						appdetect.DbMongo,
					},
				},
			},
			interactions: []string{
				"Remove a detected service",
				appdetect.DbMongo.Display(),
				"y",
				"Confirm and continue initializing my app",
			},
			want: []appdetect.Project{
				{
					Language:      appdetect.DotNet,
					Path:          dotNetDir,
					DatabaseDeps:  []appdetect.DatabaseDep{},
					DetectionRule: string(EntryKindModified),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &detectConfirm{
				console: input.NewConsole(
					false,
					false,
					os.Stdout,
					input.ConsoleHandles{
						Stderr: os.Stderr,
						Stdin:  strings.NewReader(strings.Join(tt.interactions, "\n") + "\n"),
						Stdout: os.Stdout,
					},
					nil),
			}
			d.Init(tt.detection, dir)

			err = d.Confirm(context.Background())
			require.NoError(t, err)

			require.Equal(t, tt.want, d.Services)
		})
	}
}
