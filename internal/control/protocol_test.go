package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestControlProtocolValid(t *testing.T) {
	t.Parallel()
	profile := ProfileStatus{Source: "imported", SourceRef: "stored-source", ProfileIndex: 1}
	status := Status{SchemaVersion: 1, Generation: 1, EventNodes: []string{}, DegradedReasons: []Diagnostic{}}
	for _, tc := range []struct {
		method Method
		params Params
		result Result
	}{
		{MethodShow, Params{}, VisibilityResult{Visible: true}},
		{MethodHide, Params{}, VisibilityResult{}},
		{MethodToggle, Params{}, VisibilityResult{Visible: true}},
		{MethodReload, Params{}, ReloadResult{Accepted: true, RequestGeneration: 2}},
		{MethodStatus, Params{}, status},
		{MethodSelect, Params{Selector: "Profile\nName"}, SelectionResult{ActiveProfile: profile, Generation: 2}},
		{MethodQuit, Params{}, QuitResult{Quitting: true}},
	} {
		t.Run(string(tc.method), func(t *testing.T) {
			request := Request{Version: 1, ID: "42", Method: tc.method, Params: tc.params}
			wire, err := EncodeRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := ReadRequest(bytes.NewReader(wire))
			if err != nil || decoded != request {
				t.Fatalf("request = %+v, %v", decoded, err)
			}
			response := SuccessResponse(request, tc.result)
			wire, err = EncodeResponse(response)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadResponse(bytes.NewReader(wire), request)
			if err != nil || !reflect.DeepEqual(got, response) {
				t.Fatalf("response = %+v, %v", got, err)
			}
		})
	}
	t.Run("exact limit and only first line", func(t *testing.T) {
		line := `{"version":1,"id":"42","method":"status","params":{}}`
		line += strings.Repeat(" ", MaxLineBytes-len(line)) + "\n" + `{"bad":true}` + "\n"
		request, err := ReadRequest(strings.NewReader(line))
		if err != nil || request.ID != "42" || request.Method != MethodStatus {
			t.Fatalf("request = %+v, %v", request, err)
		}
	})
	t.Run("safe failure", func(t *testing.T) {
		request := Request{Version: 1, ID: "42", Method: MethodSelect}
		response := FailureResponse(&request.ID, NewError(ERR_PROFILE_AMBIGUOUS))
		wire, err := EncodeResponse(response)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadResponse(bytes.NewReader(wire), request)
		if err != nil || !reflect.DeepEqual(got, response) {
			t.Fatalf("response = %+v, %v", got, err)
		}
	})
	t.Run("oversized response replaced before writing", func(t *testing.T) {
		name := strings.Repeat("private-marker", MaxLineBytes)
		request := Request{Version: 1, ID: "42", Method: MethodSelect}
		profile.Name = &name
		wire, err := EncodeResponse(SuccessResponse(request, SelectionResult{ActiveProfile: profile, Generation: 2}))
		if err != nil || len(wire) > MaxLineBytes+1 || bytes.Contains(wire, []byte("private-marker")) {
			t.Fatalf("unsafe response: length=%d, error=%v", len(wire), err)
		}
		got, err := ReadResponse(bytes.NewReader(wire), request)
		if err != nil || got.OK || got.Error == nil || got.Error.Code != ERR_CONTROL_TOO_LARGE {
			t.Fatalf("response = %+v, %v", got, err)
		}
	})
}

func TestControlProtocolInvalid(t *testing.T) {
	t.Parallel()
	valid := `{"version":1,"id":"42","method":"status","params":{}}`
	for _, tc := range []struct{ name, input, code string }{
		{"no newline", valid, ERR_CONTROL_REQUEST},
		{"oversize", valid + strings.Repeat(" ", MaxLineBytes-len(valid)+1) + "\n", ERR_CONTROL_TOO_LARGE},
		{"oversize without newline", strings.Repeat("x", MaxLineBytes+1), ERR_CONTROL_TOO_LARGE},
		{"malformed", `{"private-marker":` + "\n", ERR_CONTROL_REQUEST},
		{"two values", valid + " {}\n", ERR_CONTROL_REQUEST},
		{"duplicate key", `{"version":1,"id":"42","\u0069d":"private-marker","method":"status","params":{}}` + "\n", ERR_CONTROL_REQUEST},
		{"unknown field", strings.TrimSuffix(valid, "}") + `,"private-marker":1}` + "\n", ERR_CONTROL_REQUEST},
		{"missing params", `{"version":1,"id":"42","method":"status"}` + "\n", ERR_CONTROL_REQUEST},
		{"null params", strings.Replace(valid, "{}", "null", 1) + "\n", ERR_CONTROL_REQUEST},
		{"array params", strings.Replace(valid, "{}", "[]", 1) + "\n", ERR_CONTROL_REQUEST},
		{"nonempty params", strings.Replace(valid, "{}", `{"selector":"private-marker"}`, 1) + "\n", ERR_CONTROL_REQUEST},
		{"null id", strings.Replace(valid, `"42"`, "null", 1) + "\n", ERR_CONTROL_REQUEST},
		{"numeric id", strings.Replace(valid, `"42"`, "42", 1) + "\n", ERR_CONTROL_REQUEST},
		{"null version", strings.Replace(valid, ":1", ":null", 1) + "\n", ERR_CONTROL_REQUEST},
		{"wrong version", strings.Replace(valid, ":1", ":2", 1) + "\n", ERR_CONTROL_VERSION},
		{"unknown method", strings.Replace(valid, "status", "private-marker", 1) + "\n", ERR_CONTROL_METHOD},
		{"empty selector", `{"version":1,"id":"42","method":"profile.select","params":{"selector":""}}` + "\n", ERR_CONTROL_REQUEST},
		{"duplicate selector", `{"version":1,"id":"42","method":"profile.select","params":{"selector":"x","selector":"private-marker"}}` + "\n", ERR_CONTROL_REQUEST},
		{"invalid UTF8", strings.Replace(valid, "42", string([]byte{255}), 1) + "\n", ERR_CONTROL_REQUEST},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadRequest(strings.NewReader(tc.input))
			assertControlError(t, err, tc.code)
		})
	}
	t.Run("retain validated id only", func(t *testing.T) {
		request, err := ReadRequest(strings.NewReader(strings.Replace(valid, ":1", ":2", 1) + "\n"))
		if err == nil || request.ID != "42" {
			t.Fatalf("request = %+v, %v", request, err)
		}
		request, err = ReadRequest(strings.NewReader(`{"id":"42","id":"43"}` + "\n"))
		if err == nil || request.ID != "" {
			t.Fatalf("request = %+v, %v", request, err)
		}
	})
	request := Request{Version: 1, ID: "42", Method: MethodToggle}
	response := `{"version":1,"id":"42","ok":true,"result":{"visible":true},"error":null}`
	for name, input := range map[string]string{
		"wrong id":               strings.Replace(response, `"42"`, `"43"`, 1),
		"null id":                strings.Replace(response, `"42"`, "null", 1),
		"missing success":        strings.Replace(response, `"ok":true,`, "", 1),
		"null success":           strings.Replace(response, `"ok":true`, `"ok":null`, 1),
		"null result":            strings.Replace(response, `{"visible":true}`, "null", 1),
		"missing visibility":     strings.Replace(response, `{"visible":true}`, `{}`, 1),
		"null visibility":        strings.Replace(response, `{"visible":true}`, `{"visible":null}`, 1),
		"duplicate result field": strings.Replace(response, `{"visible":true}`, `{"visible":true,"visible":false}`, 1),
		"wrong result type":      strings.Replace(response, `{"visible":true}`, `{"visible":"private-marker"}`, 1),
		"mixed failure":          strings.Replace(response, `"ok":true`, `"ok":false`, 1),
		"unknown result field":   strings.Replace(response, `{"visible":true}`, `{"visible":true,"private-marker":1}`, 1),
	} {
		t.Run("response "+name, func(t *testing.T) {
			_, err := ReadResponse(strings.NewReader(input+"\n"), request)
			assertControlError(t, err, ERR_CONTROL_REQUEST)
		})
	}
	t.Run("remote error text is not trusted", func(t *testing.T) {
		response := FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
		response.Error.Summary = "private-marker"
		wire, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ReadResponse(bytes.NewReader(append(wire, '\n')), request)
		assertControlError(t, err, ERR_CONTROL_REQUEST)
	})
}

func TestControlProtocolOverlayStatus(t *testing.T) {
	t.Parallel()
	request := Request{Version: 1, ID: "42", Method: MethodStatus}
	base := Status{SchemaVersion: 1, Generation: 1, EventNodes: []string{}, DegradedReasons: []Diagnostic{}}
	oldWire, err := EncodeResponse(SuccessResponse(request, base))
	if err != nil {
		t.Fatal(err)
	}
	old, err := ReadResponse(bytes.NewReader(oldWire), request)
	if err != nil || old.Result.(Status).Overlay != nil {
		t.Fatalf("old status: %+v, %v", old, err)
	}
	nullWire := bytes.Replace(oldWire, []byte(`"visible":false`), []byte(`"visible":false,"overlay":null`), 1)
	missing, err := ReadResponse(bytes.NewReader(nullWire), request)
	if err != nil || missing.Result.(Status).Overlay != nil {
		t.Fatalf("null overlay: %+v, %v", missing, err)
	}
	base.Overlay = &OverlayStatus{Mapped: true, InputRegionApplied: false}
	wire, err := EncodeResponse(SuccessResponse(request, base))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadResponse(bytes.NewReader(wire), request)
	if err != nil || !reflect.DeepEqual(got.Result, base) {
		t.Fatalf("new status: %+v, %v", got, err)
	}
	for name, overlay := range map[string]string{
		"missing mapped":     `{"input_region_applied":false}`,
		"missing applied":    `{"mapped":true}`,
		"null mapped":        `{"mapped":null,"input_region_applied":false}`,
		"wrong applied type": `{"mapped":true,"input_region_applied":"false"}`,
		"unknown field":      `{"mapped":true,"input_region_applied":false,"private":1}`,
		"duplicate mapped":   `{"mapped":true,"mapped":false,"input_region_applied":false}`,
		"non-object":         `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			bad := bytes.Replace(wire, []byte(`{"mapped":true,"input_region_applied":false}`), []byte(overlay), 1)
			if bytes.Equal(bad, wire) {
				t.Fatal("fixture replacement did not match")
			}
			_, err := ReadResponse(bytes.NewReader(bad), request)
			assertControlError(t, err, ERR_CONTROL_REQUEST)
		})
	}
}

func assertControlError(t *testing.T, err error, code string) {
	t.Helper()
	var controlError *Error
	if !errors.As(err, &controlError) || controlError.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	wire, marshalErr := json.Marshal(controlError)
	if marshalErr != nil || bytes.Contains(wire, []byte("private-marker")) {
		t.Fatalf("unsafe error %s, %v", wire, marshalErr)
	}
}
