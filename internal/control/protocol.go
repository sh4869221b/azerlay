package control

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const ProtocolVersion = 1
const MaxLineBytes = 65_536

type Method string

const (
	MethodShow   Method = "overlay.show"
	MethodHide   Method = "overlay.hide"
	MethodToggle Method = "overlay.toggle"
	MethodReload Method = "config.reload"
	MethodStatus Method = "status"
	MethodSelect Method = "profile.select"
	MethodQuit   Method = "quit"
)

type Params struct {
	Selector string `json:"selector,omitempty"`
}

type Request struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Method  Method `json:"method"`
	Params  Params `json:"params"`
}

// Result is the closed set of control result DTOs.
type Result interface{ controlResult() }

type VisibilityResult struct {
	Visible bool `json:"visible"`
}
type ReloadResult struct {
	Accepted          bool   `json:"accepted"`
	RequestGeneration uint64 `json:"request_generation"`
}
type SelectionResult struct {
	ActiveProfile ProfileStatus `json:"active_profile"`
	Generation    uint64        `json:"generation"`
}
type QuitResult struct {
	Quitting bool `json:"quitting"`
}

func (VisibilityResult) controlResult() {}
func (ReloadResult) controlResult()     {}
func (Status) controlResult()           {}
func (SelectionResult) controlResult()  {}
func (QuitResult) controlResult()       {}

type Response struct {
	Version int     `json:"version"`
	ID      *string `json:"id"`
	OK      bool    `json:"ok"`
	Result  Result  `json:"result"`
	Error   *Error  `json:"error"`
}

func SuccessResponse(request Request, result Result) Response {
	return Response{Version: ProtocolVersion, ID: &request.ID, OK: true, Result: result}
}

func FailureResponse(id *string, err *Error) Response {
	return Response{Version: ProtocolVersion, ID: id, Error: err}
}

// readLine consumes at most one bounded LF-terminated message. The transport owns
// the one-request connection lifecycle; bytes after the first LF are not parsed.
func readLine(reader io.Reader) ([]byte, error) {
	line, err := bufio.NewReaderSize(reader, MaxLineBytes+1).ReadSlice('\n')
	if len(line) > MaxLineBytes && (len(line) != MaxLineBytes+1 || line[len(line)-1] != '\n') || errors.Is(err, bufio.ErrBufferFull) {
		return nil, NewError(ERR_CONTROL_TOO_LARGE)
	}
	if errors.Is(err, io.EOF) {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	if err != nil {
		return nil, err
	}
	line = line[:len(line)-1]
	if !utf8.Valid(line) {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	return line, nil
}

func ReadRequest(reader io.Reader) (Request, error) {
	line, err := readLine(reader)
	if err != nil {
		return Request{}, err
	}
	return decodeRequest(line)
}

func decodeRequest(line []byte) (Request, error) {
	fields, err := object(line, "version", "id", "method", "params")
	if err != nil {
		return Request{}, err
	}
	var request Request
	if json.Unmarshal(fields["id"], &request.ID) != nil || request.ID == "" {
		return Request{}, NewError(ERR_CONTROL_REQUEST)
	}
	// Preserve only a uniquely decoded, nonempty ID on subsequent failures.
	if null(fields["version"]) || json.Unmarshal(fields["version"], &request.Version) != nil {
		return request, NewError(ERR_CONTROL_REQUEST)
	}
	if request.Version != ProtocolVersion {
		return request, NewError(ERR_CONTROL_VERSION)
	}
	if null(fields["method"]) || json.Unmarshal(fields["method"], &request.Method) != nil {
		return request, NewError(ERR_CONTROL_REQUEST)
	}
	switch request.Method {
	case MethodShow, MethodHide, MethodToggle, MethodReload, MethodStatus, MethodQuit:
		_, err = object(fields["params"])
	case MethodSelect:
		var params map[string]json.RawMessage
		params, err = object(fields["params"], "selector")
		if err == nil && (json.Unmarshal(params["selector"], &request.Params.Selector) != nil || request.Params.Selector == "") {
			err = NewError(ERR_CONTROL_REQUEST)
		}
	default:
		err = NewError(ERR_CONTROL_METHOD)
	}
	return request, err
}

func EncodeRequest(request Request) ([]byte, error) {
	line, err := json.Marshal(request)
	if err != nil {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	if len(line) > MaxLineBytes {
		return nil, NewError(ERR_CONTROL_TOO_LARGE)
	}
	if _, err := decodeRequest(line); err != nil {
		return nil, err
	}
	return append(line, '\n'), nil
}

// EncodeResponse finishes serialization and size checking before transport writes.
// An oversized result is replaced as a whole with a static failure response.
func EncodeResponse(response Response) ([]byte, error) {
	line, err := json.Marshal(response)
	if err != nil {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	if len(line) > MaxLineBytes {
		response = FailureResponse(response.ID, NewError(ERR_CONTROL_TOO_LARGE))
		line, err = json.Marshal(response)
		if err != nil {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		if len(line) > MaxLineBytes {
			line, err = json.Marshal(FailureResponse(nil, NewError(ERR_CONTROL_TOO_LARGE)))
			if err != nil {
				return nil, NewError(ERR_CONTROL_REQUEST)
			}
		}
	}
	return append(line, '\n'), nil
}

func null(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// object admits exactly the named fields, rejecting duplicates including escaped
// spellings. Nested method DTOs are decoded with their own concrete field sets.
func object(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	fields := make(map[string]json.RawMessage, len(names))
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		key, ok := token.(string)
		if !ok {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		allowed := false
		for _, name := range names {
			if key == name {
				allowed = true
				break
			}
		}
		if _, duplicate := fields[key]; duplicate || !allowed {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		fields[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) || len(fields) != len(names) {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	return fields, nil
}
