package appdetect

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaDetector_Language(t *testing.T) {
	jd := &javaDetector{}
	if jd.Language() != Java {
		t.Errorf("expected language to be Java, got %v", jd.Language())
	}
}

func TestJavaDetector_DetectProject_WithPomXml(t *testing.T) {
	jd := &javaDetector{}
	entries := []fs.DirEntry{
		mockDirEntry{name: "pom.xml"},
	}
	tempDir := t.TempDir()
	os.WriteFile(filepath.Join(tempDir, "pom.xml"), []byte(`
		<project>
			
		</project>`), 0644)
	project, err := jd.DetectProject(context.Background(), tempDir, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project == nil {
		t.Fatal("expected project to be detected, got nil")
	}
}

func TestJavaDetector_DetectProject_WithoutPomXml(t *testing.T) {
	jd := &javaDetector{}
	entries := []fs.DirEntry{
		mockDirEntry{name: "not_pom.xml"},
	}
	project, err := jd.DetectProject(context.Background(), ".", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project != nil {
		t.Fatalf("expected no project to be detected, got %v", project)
	}
}

func TestAnalyzeMavenProject_WithSubmodules(t *testing.T) {
	// Setup a temporary directory with a root pom.xml and submodule poms
	tempDir := t.TempDir()
	os.WriteFile(filepath.Join(tempDir, "pom.xml"), []byte(`
		<project>
			<modules>
				<module>submodule</module>
			</modules>
		</project>`), 0644)
	os.Mkdir(filepath.Join(tempDir, "submodule"), 0755)
	os.WriteFile(filepath.Join(tempDir, "submodule", "pom.xml"), []byte(`
		<project>
			<dependencies>
				<dependency>
					<groupId>com.mysql</groupId>
					<artifactId>mysql-connector-j</artifactId>
				</dependency>
			</dependencies>
		</project>`), 0644)

	projects, err := analyzeMavenProject(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 projects, got %d", len(projects))
	}
}

func TestAnalyzeMavenProject_WithoutSubmodules(t *testing.T) {
	// Setup a temporary directory with a root pom.xml
	tempDir := t.TempDir()
	os.WriteFile(filepath.Join(tempDir, "pom.xml"), []byte(`
		<project>
			<dependencies>
				<dependency>
					<groupId>org.postgresql</groupId>
					<artifactId>postgresql</artifactId>
				</dependency>
			</dependencies>
		</project>`), 0644)

	projects, err := analyzeMavenProject(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
}

func TestDetectDependencies_WithDatabaseDeps(t *testing.T) {
	mavenProj := &mavenProject{
		Dependencies: []dependency{
			{GroupId: "com.mysql", ArtifactId: "mysql-connector-j"},
			{GroupId: "org.postgresql", ArtifactId: "postgresql"},
		},
	}
	project := &Project{}
	project, err := detectDependencies(mavenProj, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(project.DatabaseDeps) != 2 {
		t.Fatalf("expected 2 database dependencies, got %d", len(project.DatabaseDeps))
	}
}

func TestDetectDependencies_WithoutDatabaseDeps(t *testing.T) {
	mavenProj := &mavenProject{
		Dependencies: []dependency{
			{GroupId: "com.example", ArtifactId: "example-dependency"},
		},
	}
	project := &Project{}
	project, err := detectDependencies(mavenProj, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(project.DatabaseDeps) != 0 {
		t.Fatalf("expected 0 database dependencies, got %d", len(project.DatabaseDeps))
	}
}

// Mock implementation of fs.DirEntry for testing purposes
type mockDirEntry struct {
	name string
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return false }
func (m mockDirEntry) Type() fs.FileMode          { return 0 }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }
