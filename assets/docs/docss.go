package docs

import (
	"context"
	"embed"
	"net/http"
	"path/filepath"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/router"
)

//go:embed *.yaml
var docs embed.FS

func fs(_ context.Context, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Ext(name) {
		case "yaml", ".yaml":
			w.Header().Set("Content-Type", "application/yaml")
		case "js", ".js":
			w.Header().Set("Content-Type", "application/javascript")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		http.ServeFileFS(w, r, docs, name)
	}
}

func openAPI(_ context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!doctype html>
				<html>
				<head><title>API docs</title></head>
				<body>
	    			<redoc spec-url="/openapi.yaml"></redoc>
					<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
	          	</body>
           	</html>`))
	}
}

func RegisterHandlers(ctx context.Context, mux *http.ServeMux) {
	r := router.New(mux)
	r.Get("/openapi.yaml", fs(ctx, "openapi.yaml"))
	r.Get("/docs", openAPI(ctx))
}
