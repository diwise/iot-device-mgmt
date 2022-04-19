package gui

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/gui")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {

	wwwroot := os.Getenv("GUI_WEB_ROOT")

	filesDir := http.Dir(wwwroot)
	FileServer(router, "/", filesDir)

	router.Get("/gui", NewGuiHandler(log, app))

	return router
}

func NewGuiHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {

	tmplDir := os.Getenv("GUI_TMPL_DIR")

	return func(w http.ResponseWriter, r *http.Request) {
		t := template.New("index.html")
		t, err := t.ParseFiles(filepath.Join(tmplDir, "index.html"))
	
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		devices, err := app.ListAllDevices(r.Context())

		data := struct {
			Title string
			Items []database.Device
		}{
			Title: "Devices",
			Items: devices,
		}

		if err = t.Execute(w, data); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}