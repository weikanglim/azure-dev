package appdetect

import (
	"io/fs"
)

type PythonDetector struct {
}

func (pd *PythonDetector) DisplayName() string {
	return "python"
}

func (pd *PythonDetector) DetectProject(path string, entries []fs.DirEntry) (*Project, error) {
	for _, entry := range entries {
		if entry.Name() == "pyproject.toml" || entry.Name() == "requirements.txt" {
			return &Project{
				Language:  string(Python),
				Path:      path,
				InferRule: "Inferred by presence of: " + entry.Name(),
			}, nil
		}
	}

	return nil, nil
}
