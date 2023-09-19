package client

import (
	"context"
	"testing"

	test "github.com/diwise/service-chassis/pkg/test/http"
	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/matryer/is"
)

func TestMe(t *testing.T) {
	is := is.New(t)

	s := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/token"),
		),
		test.Returns(
			response.ContentType("application/json"),
			response.Code(200),
			response.Body([]byte(TokenResponse)),
		),
	)

	ctx := context.Background()

	c, err := New(ctx, s.URL(), s.URL()+"/token", "", "")
	if err != nil {
		defer c.Close(ctx)
	}

	is.NoErr(err)
}

const TokenResponse string = `{"access_token":"testtoken","expires_in":300,"refresh_expires_in":0,"token_type":"Bearer","not-before-policy":0,"scope":"email profile"}`
