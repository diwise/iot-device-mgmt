package devices

import (
	"context"
	"errors"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func (s service) SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error {

	log := logging.GetFromContext(ctx)
	var errs []error
	for _, t := range lwm2m {
		err := s.profiles.CreateSensorProfileType(ctx, t)
		if err != nil {
			log.Debug("failed to seed lwm2m type", "name", t.Name, "urn", t.Urn)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)

}

func (s service) SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error {

	log := logging.GetFromContext(ctx)
	var errs []error
	for _, p := range profiles {
		err := s.profiles.CreateSensorProfile(ctx, p)
		if err != nil {
			log.Debug("failed to seed device profile", "decoder", p.Decoder, "name", p.Name)
			errs = append(errs, err)
		}
		log.Debug("added device profile", "name", p.Name, "decoder", p.Decoder)
	}
	return errors.Join(errs...)

}
