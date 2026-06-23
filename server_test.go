package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
)

// dummyComponent is a minimal templ.Component implementation used to exercise
// TemplRender without depending on a specific page template.
func dummyComponent(msg string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte(msg))
		return err
	})
}

func TestTemplRender_WriteContentType_SetsHTML(t *testing.T) {
	w := httptest.NewRecorder()
	tr := TemplRender{Code: http.StatusOK, Data: nil}
	tr.WriteContentType(w)
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
}

func TestTemplRender_Render_NilData_WritesOnlyStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	tr := TemplRender{Code: http.StatusNoContent, Data: nil}
	if err := tr.Render(w); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("w.Code = %d, want %d", w.Code, http.StatusNoContent)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body len = %d, want 0 (nil data should write nothing)", w.Body.Len())
	}
}

func TestTemplRender_Render_WithData_WritesComponent(t *testing.T) {
	w := httptest.NewRecorder()
	tr := TemplRender{Code: http.StatusOK, Data: dummyComponent("hello-render")}
	if err := tr.Render(w); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("w.Code = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "hello-render" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello-render")
	}
	// Content-Type should also be set by WriteContentType, which Render calls.
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
}

func TestTemplRender_Render_PreservesCustomStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	tr := TemplRender{Code: http.StatusTeapot, Data: dummyComponent("teapot")}
	if err := tr.Render(w); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("w.Code = %d, want %d", w.Code, http.StatusTeapot)
	}
}

func TestTemplRender_Instance_AcceptsTemplComponent(t *testing.T) {
	tr := &TemplRender{}
	comp := dummyComponent("from-instance")
	got := tr.Instance("anything", comp)
	if got == nil {
		t.Fatal("Instance() returned nil for a templ.Component input")
	}
	// The returned render should always use StatusOK per the implementation.
	rendered, ok := got.(*TemplRender)
	if !ok {
		t.Fatalf("Instance() returned %T, want *TemplRender", got)
	}
	if rendered.Code != http.StatusOK {
		t.Errorf("rendered.Code = %d, want %d", rendered.Code, http.StatusOK)
	}
	if rendered.Data == nil {
		t.Error("rendered.Data is nil; want the input component")
	}
}

func TestTemplRender_Instance_NonComponentReturnsNil(t *testing.T) {
	tr := &TemplRender{}
	if got := tr.Instance("name", "not-a-component"); got != nil {
		t.Errorf("Instance() with non-templ.Component = %v, want nil", got)
	}
	if got := tr.Instance("name", 12345); got != nil {
		t.Errorf("Instance() with int = %v, want nil", got)
	}
	if got := tr.Instance("name", nil); got != nil {
		t.Errorf("Instance() with nil = %v, want nil", got)
	}
}
