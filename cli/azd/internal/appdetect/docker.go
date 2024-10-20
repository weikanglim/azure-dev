package appdetect

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func detectDocker(path string, entries []fs.DirEntry) (*Docker, error) {
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == "dockerfile" {
			dockerFilePath := filepath.Join(path, entry.Name())
			file, err := os.Open(dockerFilePath)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)

			var ports []Port
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "EXPOSE") {
					parsedPorts, _ := parsePorts(line[len("EXPOSE"):])
					ports = append(ports, parsedPorts...)
				}
			}

			return &Docker{
				Path:  dockerFilePath,
				Ports: ports,
			}, nil
		}
	}

	return nil, nil
}

func parsePorts(s string) ([]Port, error) {
	s = strings.TrimSpace(s)
	var ports []Port
	portSpecs := strings.Split(s, " ")
	for _, portSpec := range portSpecs {
		if strings.Contains(portSpec, "/") {
			parts := strings.Split(portSpec, "/")
			portNumber, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, err
			}
			protocol := parts[1]
			ports = append(ports, Port{portNumber, protocol})
		} else {
			portNumber, err := strconv.Atoi(portSpec)
			if err != nil {
				return nil, err
			}
			ports = append(ports, Port{portNumber, "tcp"})
		}
	}
	return ports, nil
}
