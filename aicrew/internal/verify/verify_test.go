package verify

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTemplateAndStages(t *testing.T) {
	p, err := ParseProfile(json.RawMessage(`{"template":"flutter"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Image != "aicrew-agent-flutter:local" || len(p.Steps) != 5 {
		t.Fatalf("template: %+v", p)
	}
	attempt, merge := p.ForStage(StageAttempt), p.ForStage(StageMerge)
	if len(attempt.Steps) != 4 || len(merge.Steps) != 5 {
		t.Fatalf("flutter build only when merging (decision 7.4/3): attempt %d, merge %d", len(attempt.Steps), len(merge.Steps))
	}
	for _, s := range attempt.Steps {
		if strings.Contains(strings.Join(s.Argv, " "), "build web") {
			t.Fatal("no build in attempts")
		}
	}
}

func TestExplicitStepsOverrideTemplate(t *testing.T) {
	p, _ := ParseProfile(json.RawMessage(`{"template":"go","steps":[{"name":"x","argv":["go","test"]}]}`))
	if len(p.Steps) != 1 || p.Image != "aicrew-agent-go:local" {
		t.Fatalf("%+v", p)
	}
}

func TestBadProfilesAreRejected(t *testing.T) {
	for _, raw := range []string{
		`{"template":"cobol"}`,
		`{"steps":[{"name":"x"}]}`,
		`{"steps":[{"name":"x","argv":["a"],"stages":["night"]}]}`,
		`{"steps":[{"name":"x","argv":["a"],"report":"junit"}]}`,
	} {
		if _, err := ParseProfile(json.RawMessage(raw)); err == nil {
			t.Errorf("must reject %s", raw)
		}
	}
}

func TestFlutterMachineReport(t *testing.T) {
	out := strings.Join([]string{
		`{"protocolVersion":"0.1.1","runnerVersion":"1.25.8","type":"start"}`,
		`{"test":{"id":1,"name":"loading /work/test/widget_test.dart"},"type":"testStart"}`,
		`{"testID":1,"result":"success","hidden":true,"skipped":false,"type":"testDone"}`,
		`{"test":{"id":3,"name":"adds a todo"},"type":"testStart"}`,
		`{"testID":3,"result":"success","hidden":false,"skipped":false,"type":"testDone"}`,
		`{"test":{"id":4,"name":"deletes a todo"},"type":"testStart"}`,
		`{"testID":4,"error":"Expected: exactly one matching candidate\n  Actual: _TextWidgetFinder:<Found 0 widgets>","stackTrace":"package:flutter_test/src/widget_tester.dart 469:3\ntest/widget_test.dart 52:5","isFailure":true,"type":"error"}`,
		`{"testID":4,"result":"failure","hidden":false,"skipped":false,"type":"testDone"}`,
		`{"test":{"id":5,"name":"skipped one"},"type":"testStart"}`,
		`{"testID":5,"result":"success","hidden":false,"skipped":true,"type":"testDone"}`,
		`{"success":false,"type":"done"}`,
	}, "\n")
	r := ParseFlutterMachine(out)
	if r == nil || r.Total != 3 || r.Passed != 1 || r.Failed != 1 || r.Skipped != 1 {
		t.Fatalf("%+v", r)
	}
	if len(r.Failures) != 1 || r.Failures[0].Name != "deletes a todo" || !strings.Contains(r.Failures[0].Message, "Found 0 widgets") {
		t.Fatalf("failures: %+v", r.Failures)
	}
	if s := r.Summary(); !strings.Contains(s, "FAILED deletes a todo") || !strings.Contains(s, "widget_test.dart 52:5") {
		t.Fatal(s)
	}
}

func TestFlutterCompileErrorHasNoReport(t *testing.T) {
	if r := ParseFlutterMachine("lib/main.dart:3:1: Error: Expected ';'"); r != nil {
		t.Fatal("no events — no report; the raw output stays in the step")
	}
}

func TestGoJSONReport(t *testing.T) {
	out := strings.Join([]string{
		`{"Action":"run","Package":"m/a","Test":"TestOK"}`,
		`{"Action":"pass","Package":"m/a","Test":"TestOK"}`,
		`{"Action":"run","Package":"m/a","Test":"TestBad"}`,
		`{"Action":"output","Package":"m/a","Test":"TestBad","Output":"    a_test.go:9: got 2, want 3\n"}`,
		`{"Action":"run","Package":"m/a","Test":"TestBad/sub"}`,
		`{"Action":"fail","Package":"m/a","Test":"TestBad/sub"}`,
		`{"Action":"fail","Package":"m/a","Test":"TestBad"}`,
		`{"Action":"fail","Package":"m/a"}`,
		`{"Action":"fail","Package":"m/broken"}`,
	}, "\n")
	r := ParseGoJSON(out)
	if r == nil || r.Total != 3 || r.Passed != 1 || r.Failed != 2 {
		t.Fatalf("%+v", r)
	}
	if r.Failures[0].Name != "m/a.TestBad" || !strings.Contains(r.Failures[0].Message, "want 3") || r.Failures[1].Name != "m/broken" {
		t.Fatalf("failures: %+v", r.Failures)
	}
}
