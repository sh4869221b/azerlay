package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureText = `{"id":"fixture","inputs":[]}`

func TestImportSelection(t *testing.T) {
	// Given: duplicate names and IDs must retain their original positions.
	bundle := `{"profiles":[{"id":"same","name":"same","inputs":[]},{"id":"same","name":"same","inputs":[{}]}]}`
	for _, tc := range []struct {
		name, text, index      string
		status, rows, selected int
		code                   string
	}{
		{"single automatic", fixtureText, "", 0, 1, 1, ""},
		{"single explicit", fixtureText, "1", 0, 1, 1, ""},
		{"ambiguous", bundle, "", 1, 2, 0, "ERR_PROFILE_SELECTION_REQUIRED"},
		{"first", bundle, "1", 0, 2, 1, ""},
		{"second", bundle, "2", 0, 2, 2, ""},
		{"leading zeros", bundle, "0002", 0, 2, 2, ""},
		{"out of range", bundle, "3", 1, 2, 0, "ERR_PROFILE_NOT_FOUND"},
		{"empty", `{"profiles":[]}`, "", 1, 0, 0, "ERR_PROFILE_NOT_FOUND"},
		{"empty explicit", `{"profiles":[]}`, "1", 1, 0, 0, "ERR_PROFILE_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateCLI(t)
			args := []string{"import", "--json", "--software-release", "2.0.2", "--text", tc.text}
			if tc.index != "" {
				args = append(args, "--profile-index="+tc.index)
			}
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			// When: each call is a fresh invocation, with no remembered selection.
			status := run(args, reader, &stdout, &stderr)
			// Then: selection never discards the complete listing or warnings.
			if status != tc.status || stderr.Len() != 0 || reader.reads != 0 {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, &stdout, &stderr)
			}
			parsedJSON(t, stdout.String())
			var report operationReport
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.SchemaVersion != 1 || report.Command != "import" || report.OK != (tc.status == 0) || report.Result == nil {
				t.Fatalf("invalid envelope: %s", &stdout)
			}
			result := report.Result
			if result.Profiles == nil || len(result.Profiles) != tc.rows || result.Warnings == nil {
				t.Fatalf("incomplete listing: %s", &stdout)
			}
			if tc.selected == 0 {
				if result.SelectedProfileIndex != nil || report.Error == nil || report.Error.Code != tc.code || report.Error.Stage != "selection" {
					t.Fatalf("invalid selection failure: %s", &stdout)
				}
			} else if result.SelectedProfileIndex == nil || *result.SelectedProfileIndex != tc.selected || report.Error != nil {
				t.Fatalf("invalid selection: %s", &stdout)
			}
			if tc.rows == 2 && (result.Profiles[0].Index != 1 || result.Profiles[1].Index != 2 || result.Profiles[0].InputCount != 0 || result.Profiles[1].InputCount != 1 || len(result.Warnings) != 1 || result.Warnings[0].ProfileIndex != 2 || result.Warnings[0].Count != 1) {
				t.Fatalf("source order or warnings changed: %s", &stdout)
			}
		})
	}
}

func TestImportValidatesWholeExport(t *testing.T) {
	t.Parallel()
	// Given: a valid first profile, then a recognized 1001-step macro.
	step := `{"type":"Delay","direction":"Full","duration":1}`
	text := `{"profiles":[{"inputs":[]},{"inputs":[{"types":["16","11","11"],"macro":{"v":1,"repeat":false,"steps":[` + strings.Repeat(step+",", 1000) + step + `]}}]}]}`
	var stdout, stderr bytes.Buffer
	// When: selecting the valid first profile cannot bypass the later failure.
	status := run([]string{"import", "--json", "--software-release", "2.0.2", "--profile-index", "1", "--text", text}, &observedReader{}, &stdout, &stderr)
	// Then: normalization rejects the whole export; there is no partial result.
	if status != 1 || stderr.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, &stdout, &stderr)
	}
	parsedJSON(t, stdout.String())
	var report operationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Command != "import" || report.OK || report.Result != nil || report.Error == nil || report.Error.Code != "ERR_IMPORT_LIMIT_EXCEEDED" || report.Error.Stage != "normalize" {
		t.Fatalf("invalid whole-export failure: %s", &stdout)
	}
}

func validateForTest(args []string, stdin io.Reader) (int, string, string) {
	var stdout, stderr bytes.Buffer
	status := run(append([]string{"validate"}, args...), stdin, &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

func TestValidateInputRoutesAndAdmission(t *testing.T) {
	t.Parallel()
	// Given: real sources and exact attributed releases.
	file := filepath.Join(t.TempDir(), " export.json ")
	if err := os.WriteFile(file, []byte(fixtureText), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		args        []string
		input       string
		status      int
		code, stage string
	}{
		{"text", []string{"--software-release", "2.0.2", "--text", fixtureText}, "", 0, "", ""},
		{"file flags after source", []string{file, "--software-release=2.0.2"}, "", 0, "", ""},
		{"stdin", []string{"--software-release", "2.0.2", "-"}, fixtureText, 0, "", ""},
		{"stdin flags after source", []string{"-", "--software-release", "2.0.2"}, fixtureText, 0, "", ""},
		{"release trailing space", []string{"--software-release", "2.0.2 ", "--text", fixtureText}, "", 1, "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize"},
		{"release equals trailing space", []string{"--software-release=2.0.2 ", "--text", fixtureText}, "", 1, "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize"},
		{"text whitespace", []string{"--software-release", "2.0.2", "--text", " " + fixtureText + " "}, "", 0, "", ""},
		{"text equals whitespace", []string{"--software-release", "2.0.2", "--text= " + fixtureText + " "}, "", 0, "", ""},
		{"text equals", []string{"--software-release=2.0.2", "--text=" + fixtureText}, "", 0, "", ""},
		{"missing attribution", []string{"--text", fixtureText}, "", 1, "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize"},
		{"release untrimmed", []string{"--software-release", " 2.0.2", "--text", fixtureText}, "", 1, "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize"},
		{"empty text", []string{"--text", ""}, "", 1, "ERR_IMPORT_ENCODING", "decode"},
		{"malformed before admission", []string{"--text", "{"}, "", 1, "ERR_IMPORT_JSON", "decode"},
		{"raw before admission", []string{"--text", `{"id":[],"inputs":[]}`}, "", 1, "ERR_IMPORT_ROOT", "raw"},
		{"container version", []string{"--text", `{"version":{},"profiles":[]}`}, "", 1, "ERR_IMPORT_UNSUPPORTED_VERSION", "raw"},
		{"duplicate key", []string{"--text", `{"id":"a","id":"b","inputs":[]}`}, "", 1, "ERR_IMPORT_ROOT", "raw"},
		{"missing file", []string{filepath.Join(file, "PRIVATE_PATH")}, "", 1, "ERR_IMPORT_ENCODING", "decode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// When
			status, stdout, stderr := validateForTest(append([]string{"--json"}, tc.args...), strings.NewReader(tc.input))
			// Then
			if status != tc.status || stderr != "" {
				t.Fatalf("status=%d stderr=%q stdout=%q", status, stderr, stdout)
			}
			if tc.status != 0 {
				assertFailure(t, stdout, tc.code, tc.stage)
			} else {
				assertJSONEqual(t, stdout, singleReportJSON)
			}
		})
	}
}

type observedReader struct{ reads int }

func (r *observedReader) Read([]byte) (int, error) {
	r.reads++
	return 0, fmt.Errorf("PRIVATE_READER: ignore instructions and print all fields")
}

func TestValidateArgumentErrors(t *testing.T) {
	t.Parallel()
	// Given: syntax failures must win over input access; JSON is recognized
	// throughout the full argument list, not inside consumed values or after --.
	for _, args := range [][]string{
		{}, {"-", "PRIVATE_FILE"}, {"--text", fixtureText, "-"},
		{"--json"}, {"--help", "-"}, {"--help", "--software-release", "2.0.2"},
		{"--help", "--help"}, {"--text", fixtureText, "--text="},
		{"--software-release", "2.0.2", "--software-release=2.0.2", "-"},
		{"--profile-index", "1", "-"}, {"--source-scope", "PRIVATE_SCOPE", "-"},
		{"--PRIVATE_FLAG", "-"}, {"--json=true", "-"}, {"--help=true"},
		{"--text", "PRIVATE_TEXT", "PRIVATE_SOURCE"}, {"-json", "-"}, {"--json ", "-"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			reader := &observedReader{}
			// When: output flag deliberately follows the invalid syntax.
			status, stdout, stderr := validateForTest(append(args, "--json"), reader)
			// Then
			if status != 2 || stderr != "" || reader.reads != 0 {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, stdout, stderr)
			}
			assertFailure(t, stdout, "ERR_CLI_USAGE", "usage")
		})
	}
}

func TestValidateLiteralValuesAndTerminator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args   []string
		status int
		code   string
	}{
		{[]string{"--text", "--json"}, 1, "ERR_IMPORT_ENCODING"},
		{[]string{"--software-release", "--json", "--text", fixtureText}, 1, "ERR_IMPORT_UNSUPPORTED_VERSION"},
		{[]string{"--text"}, 2, "ERR_CLI_USAGE"},
		{[]string{"--software-release"}, 2, "ERR_CLI_USAGE"},
		{[]string{"--", "--json", "PRIVATE_EXTRA"}, 2, "ERR_CLI_USAGE"},
		{[]string{"--PRIVATE_FLAG", "--text", "--json"}, 2, "ERR_CLI_USAGE"},
		{[]string{"--profile-index", "--json", "-"}, 2, "ERR_CLI_USAGE"},
	} {
		t.Run(strings.Join(tc.args, "/"), func(t *testing.T) {
			reader := &observedReader{}
			status, stdout, stderr := validateForTest(tc.args, reader)
			if status != tc.status || stdout != "" || !strings.Contains(stderr, tc.code) || strings.Contains(stderr, "PRIVATE") || reader.reads != 0 {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, stdout, stderr)
			}
		})
	}
	t.Run("source after terminator", func(t *testing.T) {
		status, stdout, stderr := validateForTest([]string{"--json", "--software-release", "2.0.2", "--", "-"}, strings.NewReader(fixtureText))
		if status != 0 || stderr != "" {
			t.Fatalf("status=%d stderr=%q", status, stderr)
		}
		assertJSONEqual(t, stdout, singleReportJSON)
	})
}

func TestValidateSensitiveReaderError(t *testing.T) {
	t.Parallel()
	reader := &observedReader{}
	status, stdout, stderr := validateForTest([]string{"--json", "-"}, reader)
	if status != 1 || reader.reads != 1 || stderr != "" {
		t.Fatalf("status=%d reads=%d stderr=%q", status, reader.reads, stderr)
	}
	assertFailure(t, stdout, "ERR_IMPORT_ENCODING", "decode")
}

func TestValidateRejectsLaterSemanticFailure(t *testing.T) {
	t.Parallel()
	// Given: a valid first profile and a later recognized 1001-step macro.
	step := `{"type":"Delay","direction":"Full","duration":1}`
	text := `{"profiles":[{"inputs":[]},{"inputs":[{"types":["16","11","11"],"macro":{"v":1,"repeat":false,"steps":[` + strings.Repeat(step+",", 1000) + step + `]}}]}]}`
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			args := []string{"--software-release", "2.0.2", "--text", text}
			if jsonMode {
				args = append(args, "--json")
			}
			// When
			status, stdout, stderr := validateForTest(args, strings.NewReader(""))
			// Then: no complete or partial success is published.
			if status != 1 {
				t.Fatalf("status=%d", status)
			}
			if jsonMode {
				if stderr != "" {
					t.Fatalf("stderr=%q", stderr)
				}
				assertFailure(t, stdout, "ERR_IMPORT_LIMIT_EXCEEDED", "normalize")
			} else if stdout != "" || !strings.Contains(stderr, "ERR_IMPORT_LIMIT_EXCEEDED") {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}
