package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/open-policy-agent/opa/rego"
	"go.opentelemetry.io/otel"
)

type tenantsContextKey struct {
	name string
}

var allowedTenantsCtxKey = &tenantsContextKey{"allowed-tenants"}

var tracer = otel.Tracer("iot-agent/authz")

func NewAuthenticator(ctx context.Context, policies io.Reader) (func(http.Handler) http.Handler, error) {
	module, err := io.ReadAll(policies)
	if err != nil {
		return nil, fmt.Errorf("unable to read authz policies: %s", err.Error())
	}

	query, err := rego.New(
		rego.Query("x = data.example.authz.allow"),
		rego.Module("example.rego", string(module)),
	).PrepareForEval(ctx)

	if err != nil {
		return nil, err
	}

	logger := logging.GetFromContext(ctx)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error

			_, span := tracer.Start(r.Context(), "check-auth")
			defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

			token := r.Header.Get("Authorization")

			if token == "" || !strings.HasPrefix(token, "Bearer ") {
				err = errors.New("authorization header missing")
				logger.Info(err.Error())
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			path := strings.Split(r.URL.Path, "/")

			input := map[string]any{
				"method": r.Method,
				"path":   path[1:],
				"token":  token[7:],
			}

			results, err := query.Eval(r.Context(), rego.EvalInput(input))
			if err != nil {
				logger.Error("opa eval failed", "err", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if len(results) == 0 {
				err = errors.New("opa query could not be satisfied")
				logger.Error("auth failed", "err", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {

				binding := results[0].Bindings["x"]

				// If authz fails we will get back a single bool. Check for that first.
				allowed, ok := binding.(bool)
				if ok && !allowed {
					err = errors.New("authorization failed")
					logger.Warn(err.Error())
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// If authz succeeds we should expect a result object here
				result, ok := binding.(map[string]any)

				if !ok {
					err = errors.New("unexpected result type")
					logger.Error("opa error", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				anyt, ok1 := result["tenants"]
				t, ok2 := anyt.([]any)

				if !ok1 || !ok2 {
					err = errors.New("bad response from authz policy engine")
					logger.Error("opa error", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				tenants := make([]string, len(t))
				for idx, tenant := range t {
					tenants[idx] = tenant.(string)
				}

				ctx := context.WithValue(r.Context(), allowedTenantsCtxKey, tenants)
				r = r.WithContext(ctx)
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}, nil
}

// GetAllowedTenantsFromContext extracts the names of allowed tenants, if any, from the provided context
func GetAllowedTenantsFromContext(ctx context.Context) []string {
	tenants, ok := ctx.Value(allowedTenantsCtxKey).([]string)

	if !ok {
		return []string{}
	}

	return tenants
}

func WithAllowedTenants(ctx context.Context, tenants []string) context.Context {
	return context.WithValue(ctx, allowedTenantsCtxKey, tenants)
}
