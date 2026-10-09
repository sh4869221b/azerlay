package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Any unexpected attempt to enter native startup fails the in-process test.
// Native lifecycle behavior is checked against the real executable in cmd/azerlay.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Run(args, "dev", stdin, stdout, stderr, func(string, io.Writer, io.Writer) int {
		panic("core CLI test unexpectedly requested native startup")
	})
}

type cliOutput struct {
	status         int
	stdout, stderr string
}

func checkCLIStatus(t *testing.T, got cliOutput, status int, jsonMode bool) {
	t.Helper()
	if got.status != status || (jsonMode && got.stderr != "") {
		t.Fatalf("CLI status=%d want=%d stdout=%q stderr=%q", got.status, status, got.stdout, got.stderr)
	}
}

func cliSuccess(command, result string) string {
	return fmt.Sprintf(`{"schema_version":1,"command":%q,"ok":true,"result":%s,"error":null}`, command, result)
}

func checkCLIFailure(t *testing.T, got cliOutput, status int, command, code, stage, result string) {
	t.Helper()
	checkCLIStatus(t, got, status, true)
	value := parsedJSON(t, got.stdout)
	envelope, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("failure is not an object: %s", got.stdout)
	}
	failure, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %s", got.stdout)
	}
	for _, field := range []string{"summary", "remediation"} {
		if text, ok := failure[field].(string); !ok || text == "" {
			t.Fatalf("missing error %s: %s", field, got.stdout)
		}
		delete(failure, field)
	}
	want := fmt.Sprintf(`{"schema_version":1,"command":%q,"ok":false,"result":%s,"error":{"code":%q,"stage":%q}}`, command, result, code, stage)
	if !strings.HasSuffix(got.stdout, "\n") || !reflect.DeepEqual(value, parsedJSON(t, want)) {
		t.Fatalf("failure machine fields=%v want=%s", value, want)
	}
}

func cliRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func cliWrite(t *testing.T, path string, data []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

const cliBundle = `{"profiles":[{"id":"same","name":"duplicate","inputs":[]},{"id":"same","name":"","inputs":[]},{"inputs":[]}]}`

const cliBundleResult = `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":"duplicate","input_count":0},{"index":2,"name":"","input_count":0},{"index":3,"name":null,"input_count":0}],"selected_profile_index":null,"warnings":[]}`

const cliEmptyResult = `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[],"selected_profile_index":null,"warnings":[]}`

func cliSelected(result string, index int) string {
	return strings.Replace(result, `"selected_profile_index":null`, fmt.Sprintf(`"selected_profile_index":%d`, index), 1)
}

type cliFileState struct {
	Mode fs.FileMode
	Data []byte
}

func cliTree(t *testing.T, root string) map[string]cliFileState {
	t.Helper()
	state := make(map[string]cliFileState)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := cliFileState{Mode: info.Mode()}
		if !entry.IsDir() {
			item.Data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		state[rel] = item
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func checkCLIPrivate(t *testing.T, got cliOutput) {
	t.Helper()
	for _, sentinel := range []string{"PRIVATE_LABEL", "PRIVATE_MACRO", "PRIVATE_FIELD", "PRIVATE_ID", "PRIVATE_PATH", "PRIVATE_ARG"} {
		if strings.Contains(got.stdout, sentinel) || strings.Contains(got.stderr, sentinel) {
			t.Fatalf("private sentinel %s leaked: stdout=%q stderr=%q", sentinel, got.stdout, got.stderr)
		}
	}
	if strings.ContainsRune(got.stdout+got.stderr, '\x1b') {
		t.Fatal("unescaped terminal control in output")
	}
}

func checkCLIHuman(t *testing.T, text, expected string) {
	t.Helper()
	var want struct {
		RootKind string `json:"root_kind"`
		Version  struct {
			Present bool            `json:"present"`
			Value   json.RawMessage `json:"value"`
		} `json:"export_version"`
		Profiles []struct {
			Index  int     `json:"index"`
			Name   *string `json:"name"`
			Inputs int     `json:"input_count"`
		} `json:"profiles"`
		Selected *int `json:"selected_profile_index"`
		Warnings []struct {
			Code  string `json:"code"`
			Index int    `json:"profile_index"`
			Count int    `json:"count"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	wanted := map[string][]string{"Root kind": {want.RootKind}, "Profiles": {strconv.Itoa(len(want.Profiles))}}
	version := "<missing>"
	if want.Version.Present {
		version = string(want.Version.Value)
		var value string
		if version != "null" && json.Unmarshal(want.Version.Value, &value) == nil {
			version = strconv.Quote(value)
		}
	}
	wanted["Export version"] = []string{version}
	if want.Selected != nil {
		wanted["Selected profile"] = []string{strconv.Itoa(*want.Selected)}
	}
	for _, row := range want.Profiles {
		name := "<unnamed>"
		if row.Name != nil {
			name = strconv.Quote(*row.Name)
		}
		wanted["rows"] = append(wanted["rows"], fmt.Sprintf("%d|%s|%d", row.Index, name, row.Inputs))
	}
	for _, warning := range want.Warnings {
		wanted["warnings"] = append(wanted["warnings"], fmt.Sprintf("%s|%d|%d", warning.Code, warning.Index, warning.Count))
	}
	actual := make(map[string][]string)
	rowPattern := regexp.MustCompile(`^  ([0-9]+)\. (.+) \(([0-9]+) inputs\)$`)
	warningPattern := regexp.MustCompile(`^(WARN_[A-Z_]+): profile ([0-9]+), ([0-9]+) Unknown outcomes$`)
	for _, line := range strings.Split(text, "\n") {
		if row := rowPattern.FindStringSubmatch(line); row != nil {
			actual["rows"] = append(actual["rows"], strings.Join(row[1:], "|"))
		} else if warning := warningPattern.FindStringSubmatch(line); warning != nil {
			actual["warnings"] = append(actual["warnings"], strings.Join(warning[1:], "|"))
		} else if key, value, ok := strings.Cut(line, ": "); ok {
			actual[key] = append(actual[key], value)
		}
	}
	if !reflect.DeepEqual(actual, wanted) {
		t.Fatalf("human report fields=%v want=%v; output=%q", actual, wanted, text)
	}
}
