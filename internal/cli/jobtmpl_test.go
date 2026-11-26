package cli

import (
	"strings"
	"testing"
)

// The comments of a generated job are copied into projects as examples: they
// must not drop errors (Lookup's ok, Every's error) and must not mention
// framework releases that no project of this CLI can use.
func TestJobTemplateExamplesHandleErrors(t *testing.T) {
	b, err := render("job.go.tmpl", genData{Names: Names{Pascal: "SendReport"}})
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, bad := range []string{", _ :=", "v0.3", "\trunner.Every(time.Hour"} {
		if strings.Contains(src, bad) {
			t.Errorf("job.go contains %q:\n%s", bad, src)
		}
	}
	for _, want := range []string{"runner, ok := aemeath.Lookup[*aejobs.Runner](app)", "if err := runner.Every(time.Hour, &jobs.SendReportJob{}); err != nil {"} {
		if !strings.Contains(src, want) {
			t.Errorf("job.go lacks %q", want)
		}
	}
}
