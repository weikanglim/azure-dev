package appdetect

import (
	"io/fs"
)

type JavaDetector struct {
}

func (jd *JavaDetector) Type() ProjectType {
	return Java
}

func (jd *JavaDetector) DetectProject(path string, entries []fs.DirEntry) (*Project, error) {
	for _, entry := range entries {
		if entry.Name() == "pom.xml" {
			return &Project{
				Language:  string(Java),
				Path:      path,
				InferRule: "Inferred by presence of: " + entry.Name(),
			}, nil
		}
	}

	return nil, nil
}
