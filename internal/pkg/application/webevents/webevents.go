package webevents

import (
	"encoding/json"

	gosse "github.com/alexandrevicenzi/go-sse"
)

type WebEvents interface {
	Server() *gosse.Server
	Shutdown()
	Publish(event string, data any) error
}

type webEvents struct {
	s *gosse.Server
}

func New() WebEvents {
	return &webEvents{
		s: gosse.NewServer(&gosse.Options{}),
	}
}

func (we *webEvents) Server() *gosse.Server {
	return we.s
}

func (we *webEvents) Shutdown() {
	we.s.Shutdown()
}

func (we *webEvents) Publish(event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	message := gosse.NewMessage("", string(b), event)
	we.s.SendMessage("", message)

	return nil
}

func (we *webEvents) PublishFeature(event string, data []byte) {
	message := gosse.NewMessage("", string(data), event)
	we.s.SendMessage("", message)
}
