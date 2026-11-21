package diag

import (
	"fmt"
	"testing"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// canary is a secret-shaped value (forty characters, digits, upper and lower case) that must never reach
// a record, an export, or the log file.
const canary = "Qx7Canary4Lm9Zr2Tb8Wk5Vy1Ps6Nd3Hf0Jg8RcQa"

// pipeline is the wiring the composition root builds: logger, recorder and reporter over fakes.
type pipeline struct {
	files    *fake.FS
	logger   *logx.Logger
	recorder *Recorder
	reporter *Reporter
}

func newPipeline(t *testing.T) pipeline {
	t.Helper()
	files := fake.NewFS()
	clock := fake.NewClock()
	logger, err := logx.New(files, clock, "/logs", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	recorder := NewRecorder(clock, logx.NewRedactor())
	return pipeline{files: files, logger: logger, recorder: recorder, reporter: NewReporter(logger, recorder)}
}

// secretForms returns the ways a canary can reach an error: as a keyed value, a bearer header, a URL
// query, and free text.
func secretForms() []error {
	return []error{
		fmt.Errorf("token=%s", canary),
		fmt.Errorf("request failed with Bearer %s", canary),
		fmt.Errorf("dial https://api.example.test/v1/sync?code=%s&state=ok: refused", canary),
		fmt.Errorf("unexpected body %s", canary),
	}
}

var _ safego.Reporter = (*Reporter)(nil)
