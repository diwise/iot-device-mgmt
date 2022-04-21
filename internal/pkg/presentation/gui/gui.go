package gui

import (
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {

	wwwroot := os.Getenv("GUI_WEB_ROOT")

	filesDir := http.Dir(wwwroot)
	FileServer(router, "/", filesDir)

	router.Get("/gui", NewGuiHandler(log, app))

	return router
}

func NewGuiHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.New("index.html").Parse(templ)

		if err != nil {
			log.Error().Err(err).Msg("unable to parse template")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		devices, err := app.ListAllDevices(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("unable to list all devices")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		data := struct {
			Title string
			Items []database.Device
		}{
			Title: "Devices",
			Items: devices,
		}

		if err = t.Execute(w, data); err != nil {
			log.Error().Err(err).Msg("unable to execute template")
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
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
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

const templ string = `
<!doctype html>
<html lang="en">

<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="description" content="">
  <title>Diwise Admin GUI 0.01</title>
  <!-- Bootstrap core CSS -->
  <link href="./assets/dist/css/bootstrap.min.css" rel="stylesheet">

  <style>
    .bd-placeholder-img {
      font-size: 1.125rem;
      text-anchor: middle;
      -webkit-user-select: none;
      -moz-user-select: none;
      user-select: none;
    }

    @media (min-width: 768px) {
      .bd-placeholder-img-lg {
        font-size: 3.5rem;
      }
    }
  </style>

  <!-- Custom styles for this template -->
  <link href="./assets/css/dashboard.css" rel="stylesheet">
</head>

<body>

  <header class="navbar navbar-dark sticky-top bg-dark flex-md-nowrap p-0 shadow">
    <a class="navbar-brand col-md-3 col-lg-2 me-0 px-3" href="#">Diwise</a>
    <button class="navbar-toggler position-absolute d-md-none collapsed" type="button" data-bs-toggle="collapse"
      data-bs-target="#sidebarMenu" aria-controls="sidebarMenu" aria-expanded="false" aria-label="Toggle navigation">
      <span class="navbar-toggler-icon"></span>
    </button>
    <input class="form-control form-control-dark w-100" type="text" placeholder="Search" aria-label="Search">
    <div class="navbar-nav">
      <div class="nav-item text-nowrap">
        <a class="nav-link px-3" href="#">Sign out</a>
      </div>
    </div>
  </header>

  <div class="container-fluid">
    <div class="row">
      <nav id="sidebarMenu" class="col-md-3 col-lg-2 d-md-block bg-light sidebar collapse">
        <div class="position-sticky pt-3">
          <ul class="nav flex-column">
            <li class="nav-item">
              <a class="nav-link active" aria-current="page" href="#">
                <span data-feather="home"></span>
                Devices
              </a>
            </li>
            <li class="nav-item">
              <a class="nav-link" href="#">
                <span data-feather="file"></span>
                About
              </a>
            </li>
          </ul> 
        </div>
      </nav>

      <main class="col-md-9 ms-sm-auto col-lg-10 px-md-4">     
        <h2>{{ .Title }}</h2>
        <div class="table-responsive">
          <table class="table table-striped table-sm">
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Latitude</th>
                <th scope="col">Longitude</th>
                <th scope="col">Environment</th>
                <th scope="col">Types</th>
				<th scope="col">SensorType</th>
              </tr>
            </thead>
            <tbody>
              {{ range .Items }}
              <tr>
                <td>{{ .Identity }}</td>
                <td>{{ .Latitude }}</td>
                <td>{{ .Longitude }}</td>
                <td>{{ .Environment }}</td>				
                <td>
                  {{ range .Types }}
                    {{ . }}<br/>
                  {{ end }}
                </td>
				<td>{{ .SensorType }}</td>
              </tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </main>
    </div>
  </div>


  <script src="./assets/dist/js/bootstrap.bundle.min.js"></script>

  <script src="https://cdn.jsdelivr.net/npm/feather-icons@4.28.0/dist/feather.min.js"
    integrity="sha384-uO3SXW5IuS1ZpFPKugNNWqTZRRglnUJK6UAZ/gxOX80nxEkN9NcGZTftn6RzhGWE"
    crossorigin="anonymous"></script>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@2.9.4/dist/Chart.min.js"
    integrity="sha384-zNy6FEbO50N+Cg5wap8IKA4M/ZnLJgzc6w2NqACZaK0u0FXfOWRRJOnQtpZun8ha"
    crossorigin="anonymous"></script>
  <script src="./assets/js/dashboard.js"></script>
</body>

</html>
`
