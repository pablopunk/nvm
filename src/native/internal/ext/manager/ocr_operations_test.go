package manager

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestOCRContractRequirementsShareActualServiceSupport(t *testing.T) {
	text := fake.NewTextRecognizer()
	m := &Manager{serviceDeps: services.Deps{Capabilities: platform.NewCapabilities(platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)}}
	if m.OperationsSupported([]api.Operation{api.OperationOCRImage}) || m.OperationsSupported([]api.Operation{api.OperationOCRScreen}) {
		t.Fatal("nil OCR advertised")
	}
	m.serviceDeps.OCR = ocr.New(ocr.Deps{Text: text})
	if !m.OperationsSupported([]api.Operation{api.OperationOCRImage}) || m.OperationsSupported([]api.Operation{api.OperationOCRScreen}) {
		t.Fatal("image-only OCR support lost")
	}
	m.serviceDeps.OCR = ocr.New(ocr.Deps{Text: text, Capture: fake.NewScreenCapturer(), FS: fake.NewFS()})
	if !m.OperationsSupported([]api.Operation{api.OperationOCRImage, api.OperationOCRScreen}) {
		t.Fatal("wired capture support missing")
	}
	if len(text.Calls()) != 0 {
		t.Fatal("requirements invoked recognizer")
	}
}
