package control

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestLiveStatusImmediateControl(t *testing.T) {
	m, _ := controllerManager(t, "")
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	c, err := NewController(t.Context(), m, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	rate := 25.0
	dropped := uint64(2)
	c.SetLiveStatusProvider(func() *LiveStatus { return &LiveStatus{EventRate: &rate, RenderRate: &rate, DroppedCount: &dropped} })
	initial := controllerStatus(t, c)
	c.AdvanceRuntimeGeneration()
	shown := c.Dispatch(t.Context(), controllerRequest(MethodShow, ""))
	if !shown.OK {
		t.Fatal(shown.Error)
	}
	selected := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First"))
	if !selected.OK {
		t.Fatal(selected.Error)
	}
	status := controllerStatus(t, c)
	if !status.Visible || status.ActiveProfile.ProfileIndex != 1 || status.Generation != selected.Result.(SelectionResult).Generation || status.Generation <= initial.Generation {
		t.Fatalf("immediate controls: %+v", status)
	}
	if status.EventRate == nil || *status.EventRate != 25 || status.ResyncCount != nil || len(status.EventNodes) != 0 {
		t.Fatalf("live schema: %+v", status)
	}
	request := controllerRequest(MethodStatus, "")
	var wire bytes.Buffer
	if err := json.NewEncoder(&wire).Encode(SuccessResponse(request, status)); err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadResponse(&wire, request)
	if err != nil {
		t.Fatal(err)
	}
	if result := decoded.Result.(Status); result.SchemaVersion != 1 || result.EventRate == nil || *result.EventRate != 25 || result.ResyncCount != nil {
		t.Fatalf("live status wire contract: %+v", result)
	}
	for range 5 {
		if got := controllerStatus(t, c); got.Generation != status.Generation {
			t.Fatalf("poll changed generation: %+v", got)
		}
	}
	accepted := c.Dispatch(t.Context(), controllerRequest(MethodReload, ""))
	if !accepted.OK {
		t.Fatal(accepted.Error)
	}
	if got := controllerStatus(t, c); got.LastReload.RequestGeneration < accepted.Result.(ReloadResult).RequestGeneration {
		t.Fatalf("accepted request missing: %+v", got.LastReload)
	}
}
