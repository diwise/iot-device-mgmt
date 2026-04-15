package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
	"go.opentelemetry.io/otel"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

type accessContextKey struct{ name string }

var accessCtxKey = &accessContextKey{"access"}

var tracer = otel.Tracer("iot-device-mgmt/authz")

type Scope string

var AnyScope Scope = Scope("any")

type Enticator interface {
	RequireAccess(scopes ...Scope) func(http.Handler) http.Handler
}

type accessMap map[string]map[Scope]struct{}

type impl struct {
	query rego.PreparedEvalQuery
}

func (a *impl) RequireAccess(scopes ...Scope) func(http.Handler) http.Handler {

	validate_scopes := make([]string, 0, len(scopes))
	for _, s := range scopes {
		validate_scopes = append(validate_scopes, string(s))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error

			logger := logging.GetFromContext(r.Context())

			_, span := tracer.Start(r.Context(), "check-auth")
			defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

			token := r.Header.Get("Authorization")

			if token == "" || !strings.HasPrefix(token, "Bearer ") {
				err = errors.New("authorization header missing")
				logger.Info(err.Error())
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			input := map[string]any{
				"token":  token[7:],
				"scopes": validate_scopes,
			}

			results, err := a.query.Eval(r.Context(), rego.EvalInput(input))
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

				anyAccess, ok1 := result["access"]
				access, ok2 := anyAccess.(map[string]any)

				if !ok1 || !ok2 {
					err = errors.New("bad response from authz policy engine")
					logger.Error("opa error", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				accessObj := accessMap{}

				for tenant, anyScopes := range access {
					scopes, ok := anyScopes.([]any)
					if !ok {
						logger.Error("rego response type error")
						http.Error(w, "rego error", http.StatusInternalServerError)
						return
					}

					accessObj[tenant] = map[Scope]struct{}{}

					for _, s := range scopes {
						scope := s.(string)
						accessObj[tenant][Scope(scope)] = struct{}{}
					}
				}

				if len(accessObj) == 0 {
					// requested scopes were not allowed in any tenant
					err = errors.New("authorization failed")
					logger.Warn(err.Error())
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				r = r.WithContext(WithAccess(r.Context(), accessObj))
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}
}

func NewAuthenticator(ctx context.Context, policies io.Reader) (Enticator, error) {
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

	return &impl{query: query}, nil
}

// GetTenantsWithAllowedScopes extracts the names of allowed tenants, if any, from the provided context
func GetTenantsWithAllowedScopes(ctx context.Context, scopes ...Scope) []string {
	access, ok := ctx.Value(accessCtxKey).(accessMap)
	requiredScopeCount := len(scopes)

	if !ok || requiredScopeCount == 0 {
		return []string{}
	}

	// If the required scope is AnyScope we set the scope count to
	// 0 to disable the scope checking below
	if requiredScopeCount == 1 && scopes[0] == AnyScope {
		requiredScopeCount = 0
	}

	tenants := make([]string, 0, len(access))

	for t, allowedScopes := range access {
		idx := 0

		for idx < requiredScopeCount {
			if _, ok := allowedScopes[scopes[idx]]; !ok {
				break
			}
			idx++
		}

		if idx == requiredScopeCount {
			tenants = append(tenants, t)
		}
	}

	return tenants
}

func WithAccess(ctx context.Context, access accessMap) context.Context {
	return context.WithValue(ctx, accessCtxKey, access)
}
