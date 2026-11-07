package services
import (
 "encoding/json"
 "path/filepath"
 "testing"
 "github.com/pablopunk/nvm/native/internal/ext/protocol"
)
func TestScreenshotsMissingProbe(t *testing.T) {
 _, deps := newDesktopHost(t); h := realFilesHandler(t,deps)
 path := filepath.Join(t.TempDir(),"missing.png")
 var got protocol.ThumbnailResult
 if err:=json.Unmarshal(mustOK(t,serve(t,h,protocol.ServiceDesktopFiles,protocol.OpThumbnail,protocol.PathArgs{Path:path})),&got); err!=nil {t.Fatal(err)}
 t.Logf("missing found=%v url=%q",got.Found,got.URL)
 if got.Found {t.Fatal("missing image reported found")}
}
