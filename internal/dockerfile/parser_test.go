package dockerfile

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseValidDockerfile(t *testing.T) {
	instructions, err := Parse(strings.NewReader("FROM alpine:3.22\nCOPY hello.sh /hello.sh\nRUN chmod +x /hello.sh\nCMD [\"/hello.sh\"]\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []Instruction{
		From{Image: "alpine:3.22"},
		Copy{Source: "hello.sh", Destination: "/hello.sh"},
		Run{Command: "chmod +x /hello.sh"},
		Cmd{Command: []string{"/hello.sh"}},
	}
	if !reflect.DeepEqual(instructions, want) {
		t.Fatalf("Parse() = %#v, want %#v", instructions, want)
	}
}

func TestParseSkipsBlankAndCommentLines(t *testing.T) {
	instructions, err := Parse(strings.NewReader("\n# base image\nFROM alpine:3.22\n\n# copy script\nCOPY hello.sh /hello.sh\nRUN chmod +x /hello.sh\n# entrypoint\nCMD [\"/hello.sh\"]\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []Instruction{
		From{Image: "alpine:3.22"},
		Copy{Source: "hello.sh", Destination: "/hello.sh"},
		Run{Command: "chmod +x /hello.sh"},
		Cmd{Command: []string{"/hello.sh"}},
	}
	if !reflect.DeepEqual(instructions, want) {
		t.Fatalf("Parse() = %#v, want %#v", instructions, want)
	}
}

func TestParseRejectsInvalidDockerfiles(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
		wantLine   int
		wantErr    string
	}{
		{
			name:       "empty input",
			dockerfile: "",
			wantLine:   0,
			wantErr:    "empty Dockerfile",
		},
		{
			name:       "instruction before from",
			dockerfile: "COPY hello.sh /hello.sh\nFROM alpine:3.22\n",
			wantLine:   1,
			wantErr:    "instruction before FROM",
		},
		{
			name:       "instruction before from after skipped lines",
			dockerfile: "\n# comment\n\nRUN echo hi\nFROM alpine:3.22\n",
			wantLine:   4,
			wantErr:    "instruction before FROM",
		},
		{
			name:       "duplicate from",
			dockerfile: "FROM alpine:3.22\nFROM busybox\n",
			wantLine:   2,
			wantErr:    "duplicate FROM",
		},
		{
			name:       "shell form cmd",
			dockerfile: "FROM alpine:3.22\nCMD /hello.sh\n",
			wantLine:   2,
			wantErr:    "CMD must use JSON array form",
		},
		{
			name:       "json run",
			dockerfile: "FROM alpine:3.22\nRUN [\"/bin/echo\",\"hello\"]\n",
			wantLine:   2,
			wantErr:    "RUN must use shell form",
		},
		{
			name:       "multiple copy sources",
			dockerfile: "FROM alpine:3.22\nCOPY one two /target\n",
			wantLine:   2,
			wantErr:    "COPY requires exactly one source and one destination",
		},
		{
			name:       "relative copy destination",
			dockerfile: "FROM alpine:3.22\nCOPY hello.sh hello.sh\n",
			wantLine:   2,
			wantErr:    "COPY destination must be absolute",
		},
		{
			name:       "env unsupported",
			dockerfile: "FROM alpine:3.22\nENV DEMO=1\n",
			wantLine:   2,
			wantErr:    "unsupported instruction",
		},
		{
			name:       "malformed json cmd",
			dockerfile: "FROM alpine:3.22\nCMD [\"/hello.sh\"\n",
			wantLine:   2,
			wantErr:    "invalid CMD JSON",
		},
		{
			name:       "empty cmd array",
			dockerfile: "FROM alpine:3.22\nCMD []\n",
			wantLine:   2,
			wantErr:    "CMD must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(test.dockerfile))
			if err == nil {
				t.Fatal("Parse() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Parse() error = %q, want substring %q", err.Error(), test.wantErr)
			}
			wantLine := "line " + strconv.Itoa(test.wantLine)
			if !strings.Contains(err.Error(), wantLine) {
				t.Fatalf("Parse() error = %q, want substring %q", err.Error(), wantLine)
			}
		})
	}
}

func TestParseWrapsScannerFailuresWithLineNumber(t *testing.T) {
	longLine := "FROM " + strings.Repeat("a", 70*1024)

	_, err := Parse(strings.NewReader("\n# comment\n" + longLine + "\n"))
	if err == nil {
		t.Fatal("Parse() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("Parse() error = %q, want substring %q", err.Error(), "line 3")
	}
	if !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("Parse() error = %q, want substring %q", err.Error(), "token too long")
	}
}
