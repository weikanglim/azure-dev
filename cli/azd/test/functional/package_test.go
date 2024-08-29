package cli_test

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azure/azure-dev/cli/azd/pkg/osutil"
	"github.com/azure/azure-dev/cli/azd/test/azdcli"
	"github.com/stretchr/testify/require"
)

func Test_CLI_Package_Err_WorkingDirectory(t *testing.T) {
	// running this test in parallel is ok as it uses a t.TempDir()
	t.Parallel()
	ctx, cancel := newTestContext(t)
	defer cancel()

	dir := tempDirWithDiagnostics(t)
	t.Logf("DIR: %s", dir)

	cli := azdcli.NewCLI(t)
	cli.WorkingDirectory = dir
	cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

	err := copySample(dir, "webapp")
	require.NoError(t, err, "failed expanding sample")

	_, err = cli.RunCommandWithStdIn(ctx, stdinForInit("testenv"), "init")
	require.NoError(t, err)

	// cd infra
	err = os.MkdirAll(filepath.Join(dir, "infra"), osutil.PermissionDirectory)
	require.NoError(t, err)
	cli.WorkingDirectory = filepath.Join(dir, "infra")

	result, err := cli.RunCommand(ctx, "package")
	require.Error(t, err, "package should fail in non-project and non-service directory")
	require.Contains(t, result.Stdout, "current working directory")
}

func Test_CLI_Package_FromServiceDirectory(t *testing.T) {
	// running this test in parallel is ok as it uses a t.TempDir()
	t.Parallel()
	ctx, cancel := newTestContext(t)
	defer cancel()

	dir := tempDirWithDiagnostics(t)
	t.Logf("DIR: %s", dir)

	cli := azdcli.NewCLI(t)
	cli.WorkingDirectory = dir
	cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

	err := copySample(dir, "webapp")
	require.NoError(t, err, "failed expanding sample")

	_, err = cli.RunCommandWithStdIn(ctx, stdinForInit("testenv"), "init")
	require.NoError(t, err)

	cli.WorkingDirectory = filepath.Join(dir, "src", "dotnet")

	result, err := cli.RunCommand(ctx, "package")
	require.NoError(t, err)
	require.Contains(t, result.Stdout, "Packaging service web")
}

func Test_CLI_Package_WithOutputPath(t *testing.T) {
	t.Run("AllServices", func(t *testing.T) {
		ctx, cancel := newTestContext(t)
		defer cancel()

		dir := tempDirWithDiagnostics(t)
		t.Logf("DIR: %s", dir)

		envName := randomEnvName()
		t.Logf("AZURE_ENV_NAME: %s", envName)

		cli := azdcli.NewCLI(t)
		cli.WorkingDirectory = dir
		cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

		err := copySample(dir, "webapp")
		require.NoError(t, err, "failed expanding sample")

		_, err = cli.RunCommandWithStdIn(ctx, stdinForInit(envName), "init")
		require.NoError(t, err)

		packageResult, err := cli.RunCommand(
			ctx,
			"package", "--output-path", "./dist",
		)
		require.NoError(t, err)
		require.Contains(t, packageResult.Stdout, "Package Output: dist")

		distPath := filepath.Join(dir, "dist")
		files, err := os.ReadDir(distPath)
		require.NoError(t, err)
		require.Len(t, files, 1)
	})

	t.Run("SingleService", func(t *testing.T) {
		ctx, cancel := newTestContext(t)
		defer cancel()

		dir := tempDirWithDiagnostics(t)
		t.Logf("DIR: %s", dir)

		envName := randomEnvName()
		t.Logf("AZURE_ENV_NAME: %s", envName)

		cli := azdcli.NewCLI(t)
		cli.WorkingDirectory = dir
		cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

		err := copySample(dir, "webapp")
		require.NoError(t, err, "failed expanding sample")

		_, err = cli.RunCommandWithStdIn(ctx, stdinForInit(envName), "init")
		require.NoError(t, err)

		packageResult, err := cli.RunCommand(
			ctx,
			"package", "web", "--output-path", "./dist/web.zip",
		)
		require.NoError(t, err)
		require.Contains(t, packageResult.Stdout, "Package Output: ./dist/web.zip")

		artifactPath := filepath.Join(dir, "dist", "web.zip")
		info, err := os.Stat(artifactPath)
		require.NoError(t, err)
		require.NotNil(t, info)
	})
}

func Test_CLI_Package(t *testing.T) {
	// running this test in parallel is ok as it uses a t.TempDir()
	t.Parallel()
	ctx, cancel := newTestContext(t)
	defer cancel()

	dir := tempDirWithDiagnostics(t)
	t.Logf("DIR: %s", dir)

	envName := randomEnvName()
	t.Logf("AZURE_ENV_NAME: %s", envName)

	cli := azdcli.NewCLI(t)
	cli.WorkingDirectory = dir
	cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

	err := copySample(dir, "webapp")
	require.NoError(t, err, "failed expanding sample")

	_, err = cli.RunCommandWithStdIn(ctx, stdinForInit(envName), "init")
	require.NoError(t, err)

	packageResult, err := cli.RunCommand(ctx, "package", "web")
	require.NoError(t, err)
	require.Contains(t, packageResult.Stdout, fmt.Sprintf("Package Output: %s", os.TempDir()))
}

func Test_CLI_Package_ZipIgnore(t *testing.T) {
	t.Parallel()
	ctx, cancel := newTestContext(t)
	defer cancel()

	// Set this to true if you want to print the directory and zip contents for debugging
	printDebug := false

	// Create a temporary directory for the project
	dir := tempDirWithDiagnostics(t)

	// Set up the CLI with the appropriate working directory and environment variables
	cli := azdcli.NewCLI(t)
	cli.WorkingDirectory = dir
	cli.Env = append(os.Environ(), "AZURE_LOCATION=eastus2")

	// Copy the sample project to the app directory
	err := copySample(dir, "dotignore")
	require.NoError(t, err, "failed expanding sample")

	// Print directory contents for debugging if printDebug is true
	if printDebug {
		printDirContents(t, "service1", filepath.Join(dir, "src", "service1"))
		printDirContents(t, "service2", filepath.Join(dir, "src", "service2"))
	}

	// Run the init command to initialize the project
	_, err = cli.RunCommandWithStdIn(
		ctx,
		"Use code in the current directory\n"+
			"Confirm and continue initializing my app\n"+
			"appdb\n"+
			"TESTENV\n",
		"init",
	)
	require.NoError(t, err)

	// Define the scenarios to test
	scenarios := []struct {
		name              string
		description       string
		enabled           bool
		rootZipIgnore     string
		service1ZipIgnore string
		expectedFiles     map[string]map[string]bool
	}{
		{
			name: "No zipignore",
			description: "Tests the default behavior when no .zipignore files are present. " +
				"Verifies that common directories like __pycache__, .venv, and node_modules are excluded.",
			enabled: true,
			expectedFiles: map[string]map[string]bool{
				"service1": {
					"testfile.py":               true,
					"__pycache__/testcache.txt": false,
					".venv/pyvenv.cfg":          false,
					"logs/log.txt":              true,
				},
				"service2": {
					"testfile.js":                            true,
					"node_modules/some_package/package.json": false,
					"logs/log.txt":                           true,
				},
			},
		},
		{
			name: "Root zipignore excluding pycache",
			description: "Tests the behavior when a root .zipignore excludes __pycache__.  " +
				"Verifies that __pycache__ is excluded in both services, but other directories are included.",
			enabled:       true,
			rootZipIgnore: "__pycache__\n",
			expectedFiles: map[string]map[string]bool{
				"service1": {
					"testfile.py":               true,
					"__pycache__/testcache.txt": false,
					".venv/pyvenv.cfg":          true,
					"logs/log.txt":              true,
				},
				"service2": {
					"testfile.js":                            true,
					"node_modules/some_package/package.json": true,
					"logs/log.txt":                           true,
				},
			},
		},
		{
			name: "Root and Service1 zipignore",
			description: "Tests the behavior when both the root and Service1 have .zipignore files.  " +
				"Verifies that the root .zipignore affects both services, but Service1's .zipignore " +
				"takes precedence for its own files.",
			enabled:           true,
			rootZipIgnore:     "logs/\n",
			service1ZipIgnore: "__pycache__\n",
			expectedFiles: map[string]map[string]bool{
				"service1": {
					"testfile.py":               true,
					"__pycache__/testcache.txt": false,
					".venv/pyvenv.cfg":          true,
					"logs/log.txt":              false,
				},
				"service2": {
					"testfile.js":                            true,
					"node_modules/some_package/package.json": true,
					"logs/log.txt":                           false,
				},
			},
		},
		{
			name: "Service1 zipignore only",
			description: "Tests the behavior when only Service1 has a .zipignore file. " +
				"Verifies that Service1 follows its .zipignore, while Service2 uses the default behavior.",
			enabled:           true,
			service1ZipIgnore: "__pycache__\n",
			expectedFiles: map[string]map[string]bool{
				"service1": {
					"testfile.py":               true,
					"__pycache__/testcache.txt": false,
					".venv/pyvenv.cfg":          true,
					"logs/log.txt":              true,
				},
				"service2": {
					"testfile.js":                            true,
					"node_modules/some_package/package.json": false,
					"logs/log.txt":                           true,
				},
			},
		},
	}

	for _, scenario := range scenarios {
		if !scenario.enabled {
			continue
		}

		t.Run(scenario.name, func(t *testing.T) {
			// Print the scenario description
			t.Logf("Scenario: %s - %s", scenario.name, scenario.description)

			// Set up .zipignore files based on the scenario
			if scenario.rootZipIgnore != "" {
				err := os.WriteFile(filepath.Join(dir, ".zipignore"), []byte(scenario.rootZipIgnore), 0600)
				require.NoError(t, err)
			}
			if scenario.service1ZipIgnore != "" {
				err := os.WriteFile(filepath.Join(dir, "src", "service1", ".zipignore"),
					[]byte(scenario.service1ZipIgnore), 0600)
				require.NoError(t, err)
			}

			// Print directory contents after writing .zipignore if printDebug is true
			if printDebug {
				printDirContents(t, "service1", filepath.Join(dir, "src", "service1"))
			}

			// Run the package command and specify an output path
			outputDir := filepath.Join(dir, "dist_"+strings.ReplaceAll(scenario.name, " ", "_"))
			err = os.Mkdir(outputDir, 0755) // Ensure the directory exists
			require.NoError(t, err)

			_, err = cli.RunCommand(ctx, "package", "--output-path", outputDir)
			require.NoError(t, err)

			// Print directory contents of the output directory if printDebug is true
			if printDebug {
				printDirContents(t, scenario.name+" output", outputDir)
			}

			// Verify that the package was created and the output directory exists
			files, err := os.ReadDir(outputDir)
			require.NoError(t, err)
			require.Len(t, files, 2)

			// Check contents of Service1 package
			checkServicePackage(t, outputDir, "service1", scenario.expectedFiles["service1"], printDebug)

			// Check contents of Service2 package
			checkServicePackage(t, outputDir, "service2", scenario.expectedFiles["service2"], printDebug)

			// Clean up .zipignore files and generated zip files
			os.RemoveAll(outputDir)
			os.Remove(filepath.Join(dir, ".zipignore"))
			os.Remove(filepath.Join(dir, "src", "service1", ".zipignore"))
		})
	}
}

// Helper function to check service package contents
func checkServicePackage(t *testing.T, distPath, serviceName string, expectedFiles map[string]bool, printDebug bool) {
	zipFilePath := findServiceZipFile(t, distPath, serviceName)
	if printDebug {
		printZipContents(t, serviceName, zipFilePath) // Print the contents of the zip file if printDebug is true
	}
	zipReader, err := zip.OpenReader(zipFilePath)
	require.NoError(t, err)
	defer zipReader.Close()

	checkZipContents(t, zipReader, expectedFiles, serviceName)
}

// Helper function to find the zip file by service name
func findServiceZipFile(t *testing.T, distPath, serviceName string) string {
	files, err := os.ReadDir(distPath)
	require.NoError(t, err)

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".zip" && strings.Contains(file.Name(), serviceName) {
			return filepath.Join(distPath, file.Name())
		}
	}

	t.Fatalf("Zip file for service '%s' not found", serviceName)
	return ""
}

// Helper function to print directory contents for debugging
func printDirContents(t *testing.T, serviceName, dir string) {
	t.Logf("[%s] Listing directory: %s", serviceName, dir)
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, file := range files {
		t.Logf("[%s] Found: %s", serviceName, file.Name())
		if file.IsDir() {
			printDirContents(t, serviceName,
				filepath.Join(dir, file.Name())) // Recursive call to list sub-directory contents
		}
	}
}

// Helper function to print the contents of a zip file
func printZipContents(t *testing.T, serviceName, zipFilePath string) {
	t.Logf("[%s] Listing contents of zip file: %s", serviceName, zipFilePath)
	zipReader, err := zip.OpenReader(zipFilePath)
	require.NoError(t, err)
	defer zipReader.Close()

	for _, file := range zipReader.File {
		t.Logf("[%s] Found in zip: %s", serviceName, file.Name)
	}
}

// Helper function to check zip contents against expected files
func checkZipContents(t *testing.T, zipReader *zip.ReadCloser, expectedFiles map[string]bool, serviceName string) {
	foundFiles := make(map[string]bool)

	for _, file := range zipReader.File {
		// Normalize the file name to use forward slashes
		normalizedFileName := strings.ReplaceAll(file.Name, "\\", "/")
		foundFiles[normalizedFileName] = true
	}

	for expectedFile, shouldExist := range expectedFiles {
		// Normalize the expected file name to use forward slashes
		normalizedExpectedFile := strings.ReplaceAll(expectedFile, "\\", "/")
		if shouldExist {
			if !foundFiles[normalizedExpectedFile] {
				t.Errorf("[%s] Expected file '%s' to be included in the package but it was not found",
					serviceName, normalizedExpectedFile)
			}
		} else {
			if foundFiles[normalizedExpectedFile] {
				t.Errorf("[%s] Expected file '%s' to be excluded from the package but it was found",
					serviceName, normalizedExpectedFile)
			}
		}
	}
}
