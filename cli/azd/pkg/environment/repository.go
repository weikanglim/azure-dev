package environment

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/azure/azure-dev/cli/azd/pkg/contracts"
	"github.com/azure/azure-dev/cli/azd/pkg/environment/azdcontext"
	"github.com/azure/azure-dev/cli/azd/pkg/osutil"
)

const ConfigFileName = "config.json"
const ConfigFileVersion = 1

type EnvironmentSpec struct {
	EnvironmentName string
	Subscription    string
	Location        string
}

// Repository managers CRUD for environments.
type Repository struct {
	azdCtx *azdcontext.AzdContext
}

var ErrEnvironmentExists = errors.New("environment already exists")

func (r *Repository) Directory() string {
	return r.azdCtx.EnvironmentDirectory()
}

func (r *Repository) Create(name string) error {
	if err := os.MkdirAll(r.Directory(), osutil.PermissionDirectory); err != nil {
		return fmt.Errorf("creating environment root: %w", err)
	}

	if err := os.Mkdir(filepath.Join(r.Directory(), name), osutil.PermissionDirectory); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrEnvironmentExists
		}

		return fmt.Errorf("creating environment directory: %w", err)
	}

	return nil
}

func (r *Repository) Read(name string) (*Environment, error) {
	return FromRoot(r.azdCtx.EnvironmentRoot(name))
}

func (r *Repository) Delete(name string) error {
	return os.RemoveAll(filepath.Join(r.Directory(), name))
}

func (r *Repository) List() ([]contracts.EnvListEnvironment, error) {
	defaultEnv, err := r.GetDefault()
	if err != nil {
		return nil, err
	}

	ents, err := os.ReadDir(r.azdCtx.EnvironmentDirectory())
	if err != nil {
		return nil, fmt.Errorf("listing entries: %w", err)
	}

	var envs []contracts.EnvListEnvironment
	for _, ent := range ents {
		if ent.IsDir() {
			ev := contracts.EnvListEnvironment{
				Name:       ent.Name(),
				IsDefault:  ent.Name() == defaultEnv,
				DotEnvPath: r.azdCtx.EnvironmentDotEnvPath(ent.Name()),
			}
			envs = append(envs, ev)
		}
	}

	sort.Slice(envs, func(i, j int) bool {
		return envs[i].Name < envs[j].Name
	})
	return envs, nil
}

func (r *Repository) Exists(name string) error {
	_, err := os.ReadDir(r.azdCtx.EnvironmentRoot(name))
	return err
}

type configFile struct {
	Version            int    `json:"version"`
	DefaultEnvironment string `json:"defaultEnvironment"`
}

func (r *Repository) SetDefault(name string) error {
	path := filepath.Join(r.Directory(), ConfigFileName)
	bytes, err := json.Marshal(configFile{
		Version:            ConfigFileVersion,
		DefaultEnvironment: name,
	})
	if err != nil {
		return fmt.Errorf("serializing config file: %w", err)
	}

	if err := os.WriteFile(path, bytes, osutil.PermissionFile); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

func (r *Repository) GetDefault() (string, error) {
	path := filepath.Join(r.Directory(), ConfigFileName)
	file, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("reading config file: %w", err)
	}

	var config configFile
	if err := json.Unmarshal(file, &config); err != nil {
		return "", fmt.Errorf("deserializing config file: %w", err)
	}

	return config.DefaultEnvironment, nil
}
