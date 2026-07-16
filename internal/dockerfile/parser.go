package dockerfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

type Instruction interface {
	instruction()
}

type From struct {
	Image string
}

func (From) instruction() {}

type Copy struct {
	Source      string
	Destination string
}

func (Copy) instruction() {}

type Run struct {
	Command string
}

func (Run) instruction() {}

type Cmd struct {
	Command []string
}

func (Cmd) instruction() {}

func Parse(reader io.Reader) ([]Instruction, error) {
	scanner := bufio.NewScanner(reader)
	var instructions []Instruction
	seenFrom := false
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, remainder := splitInstruction(line)
		if !seenFrom && name != "FROM" {
			return nil, lineError(lineNumber, "instruction before FROM")
		}

		switch name {
		case "FROM":
			if seenFrom {
				return nil, lineError(lineNumber, "duplicate FROM")
			}
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return nil, lineError(lineNumber, "FROM requires exactly one image")
			}
			instructions = append(instructions, From{Image: fields[1]})
			seenFrom = true
		case "COPY":
			fields := strings.Fields(line)
			if len(fields) != 3 {
				return nil, lineError(lineNumber, "COPY requires exactly one source and one destination")
			}
			if !path.IsAbs(fields[2]) {
				return nil, lineError(lineNumber, "COPY destination must be absolute")
			}
			instructions = append(instructions, Copy{Source: fields[1], Destination: fields[2]})
		case "RUN":
			command := strings.TrimSpace(remainder)
			if command == "" {
				return nil, lineError(lineNumber, "RUN requires a shell command")
			}
			if strings.HasPrefix(command, "[") {
				return nil, lineError(lineNumber, "RUN must use shell form")
			}
			instructions = append(instructions, Run{Command: command})
		case "CMD":
			payload := strings.TrimSpace(remainder)
			if !strings.HasPrefix(payload, "[") {
				return nil, lineError(lineNumber, "CMD must use JSON array form")
			}
			var command []string
			if err := json.Unmarshal([]byte(payload), &command); err != nil {
				return nil, lineError(lineNumber, fmt.Sprintf("invalid CMD JSON: %v", err))
			}
			if len(command) == 0 {
				return nil, lineError(lineNumber, "CMD must not be empty")
			}
			instructions = append(instructions, Cmd{Command: command})
		default:
			return nil, lineError(lineNumber, fmt.Sprintf("unsupported instruction: %s", name))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, lineError(lineNumber+1, err.Error())
	}
	if len(instructions) == 0 {
		return nil, lineError(0, "empty Dockerfile")
	}
	if !seenFrom {
		return nil, lineError(0, "empty Dockerfile")
	}

	return instructions, nil
}

func splitInstruction(line string) (string, string) {
	index := strings.IndexAny(line, " \t")
	if index == -1 {
		return line, ""
	}
	return line[:index], line[index+1:]
}

func lineError(lineNumber int, message string) error {
	return fmt.Errorf("line %d: %s", lineNumber, message)
}
