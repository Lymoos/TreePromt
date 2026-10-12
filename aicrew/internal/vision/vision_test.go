package vision

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompttree/aicrew/internal/api"
	"prompttree/aicrew/internal/review"
	"prompttree/aicrew/internal/runner"
)

// fakeDocker — «сборка» проходит, «снимок» кладёт PNG в каталог /out.
type fakeDocker struct{ calls [][]string }

func (f *fakeDocker) Run(_ context.Context, c runner.Command) (runner.Result, error) {
	f.calls = append(f.calls, c.Args)
	for i, a := range c.Args {
		if a == "-v" && strings.HasSuffix(c.Args[i+1], ":/out") {
			dir := strings.TrimSuffix(c.Args[i+1], ":/out")
			_ = os.WriteFile(filepath.Join(dir, "shot-412.png"), []byte("png-phone"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "shot-1280.png"), []byte("png-desktop"), 0o644)
		}
	}
	return runner.Result{}, nil
}

func TestVisionSendsScreenshotsAndParsesVerdict(t *testing.T) {
	var got struct {
		Model     string `json:"model"`
		KeepAlive int    `json:"keep_alive"`
		Messages  []struct {
			Content string   `json:"content"`
			Images  []string `json:"images"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"message":{"content":"{\"verdict\":\"reject\",\"rejection_type\":\"code_bug\",\"issues\":[{\"file\":\"screenshot\",\"severity\":\"blocker\",\"message\":\"overflow stripe on phone\"}],\"summary\":\"broken\"}"}}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: todo\n"), 0o644)
	dock := &fakeDocker{}
	v := Vision{R: dock, BuildImage: "flutter", Image: "vision", OllamaURL: srv.URL, Model: "qwen2.5vl:7b"}
	verdict, err := v.Review(context.Background(), Role, &api.Task{ID: "t1", Attempt: 1}, dir, review.Facts{Title: "Кнопка", Task: "Добавить кнопку"})
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Verdict != "reject" || verdict.Reviewer != "vision_qa:qwen2.5vl:7b" || !strings.Contains(verdict.Issues[0].Message, "overflow") {
		t.Fatalf("%+v", verdict)
	}
	if got.Model != "qwen2.5vl:7b" || got.KeepAlive != 0 || len(got.Messages[0].Images) != 2 || !strings.Contains(got.Messages[0].Content, "<untrusted>") {
		t.Fatalf("request: %+v", got)
	}
	if len(dock.calls) != 2 || !strings.Contains(strings.Join(dock.calls[0], " "), "flutter build web --release --no-web-resources-cdn") ||
		!strings.Contains(strings.Join(dock.calls[1], " "), ":/web:ro") {
		t.Fatalf("docker: %v", dock.calls)
	}
}

func TestVisionSkipsNonFlutterProjects(t *testing.T) {
	v := Vision{R: &fakeDocker{}}
	verdict, err := v.Review(context.Background(), Role, &api.Task{}, t.TempDir(), review.Facts{})
	if err != nil || verdict.Verdict != "approve" || !strings.Contains(verdict.Summary, "skipped") {
		t.Fatalf("%+v %v", verdict, err)
	}
}

func TestVisionGarbageAnswerIsEnvironmentFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"content":"looks nice!"}}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: x\n"), 0o644)
	_, err := Vision{R: &fakeDocker{}, OllamaURL: srv.URL}.Review(context.Background(), Role, &api.Task{}, dir, review.Facts{})
	if err == nil || !strings.Contains(err.Error(), "vision_unparsable") && !strings.Contains(err.Error(), "ENVIRONMENT_FAILURE") {
		t.Fatalf("%v", err)
	}
}
