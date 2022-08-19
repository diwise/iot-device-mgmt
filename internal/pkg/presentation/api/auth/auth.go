package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/open-policy-agent/opa/rego"
	"github.com/rs/zerolog"
)

func NewAuthenticator(ctx context.Context, logger zerolog.Logger, policies io.Reader) (func(http.Handler) http.Handler, error) {

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

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")

			if token == "" || !strings.HasPrefix(token, "Bearer ") {
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
				logger.Error().Err(err).Msg("opa eval failed")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if len(results) == 0 {
				err = errors.New("opa query could not be satisfied")
				logger.Error().Err(err).Msg("auth failed")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				result, ok := results[0].Bindings["x"].(bool)

				if !ok {
					err = errors.New("unexpected result type")
					logger.Error().Err(err).Msg("opa error")
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if !result {
					logger.Info().Msgf("opa result: %+v", results)

					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}, nil
}
