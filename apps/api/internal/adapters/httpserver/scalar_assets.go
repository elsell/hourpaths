package httpserver

import (
	_ "embed"
	"net/http"
)

const (
	scalarBrowserRuntimePath    = "/docs/assets/scalar-api-reference-1.44.20.js"
	scalarBrowserRuntimeVersion = "1.44.20"
	scalarBrowserRuntimeSize    = 3544608
	scalarBrowserRuntimeSHA256  = "f349c815d31be09d11e386726da989e3af50c2f1885910764b51f5b0fae9e28e"
	scalarLicensePath           = "/docs/assets/scalar-api-reference-LICENSE.txt"
)

// scalarBrowserRuntime is the exact reviewed @scalar/api-reference 1.44.20
// standalone browser build. Its immutable provenance is enforced by tests.
//
//go:embed assets/scalar-api-reference-1.44.20.js
var scalarBrowserRuntime []byte

//go:embed assets/SCALAR-LICENSE.txt
var scalarLicense []byte

func scalarBrowserRuntimeHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("ETag", `"sha256-`+scalarBrowserRuntimeSHA256+`"`)
	_, _ = w.Write(scalarBrowserRuntime)
}

func scalarLicenseHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(scalarLicense)
}
