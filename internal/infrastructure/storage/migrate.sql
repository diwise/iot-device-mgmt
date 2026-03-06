CREATE TABLE IF NOT EXISTS sensor_profiles (
	sensor_profile_id	TEXT NOT NULL,
	name 				TEXT NULL,
	decoder 			TEXT NOT NULL,
	description			TEXT NULL,
	interval 			NUMERIC NOT NULL DEFAULT 3600,
	created_on  		timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_sensor_profiles PRIMARY KEY (sensor_profile_id)
);

CREATE TABLE IF NOT EXISTS sensor_profile_types (
	sensor_profile_type_id	TEXT NOT NULL,
	name 					TEXT NULL,
	created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_sensor_profile_types PRIMARY KEY (sensor_profile_type_id)
);

CREATE TABLE IF NOT EXISTS sensors (
	sensor_id	TEXT NOT NULL,
	sensor_profile	TEXT NULL,
	created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_sensors PRIMARY KEY (sensor_id)
);

CREATE TABLE IF NOT EXISTS devices (
	device_id	TEXT 	NOT NULL,
	sensor_id	TEXT 	NULL,

	active		BOOLEAN	NOT NULL DEFAULT FALSE,

	name        TEXT 	NULL,
	description TEXT 	NULL,
	environment TEXT 	NULL,
	source      TEXT 	NULL,
	tenant		TEXT 	NOT NULL,
	location 	POINT 	NULL,

	interval 		NUMERIC NOT NULL DEFAULT 0,

	created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted     BOOLEAN DEFAULT FALSE,
	deleted_on  timestamp with time zone NULL,

	CONSTRAINT pk_devices PRIMARY KEY (device_id),
	CONSTRAINT fk_devices_sensor FOREIGN KEY (sensor_id) REFERENCES sensors (sensor_id) ON DELETE SET NULL
);

ALTER TABLE sensors ADD COLUMN IF NOT EXISTS sensor_profile TEXT NULL;
ALTER TABLE sensors DROP COLUMN IF EXISTS source;
ALTER TABLE devices DROP CONSTRAINT IF EXISTS fk_device_profiles;

DO $$
BEGIN
	IF to_regclass('public.device_profiles') IS NOT NULL THEN
		INSERT INTO sensor_profiles (sensor_profile_id, name, decoder, description, interval, created_on)
		SELECT dp.device_profile_id, dp.name, dp.decoder, dp.description, dp.interval, dp.created_on
		FROM device_profiles dp
		ON CONFLICT (sensor_profile_id) DO NOTHING;
	END IF;
END $$;

DO $$
BEGIN
	IF to_regclass('public.device_profiles_types') IS NOT NULL THEN
		INSERT INTO sensor_profile_types (sensor_profile_type_id, name, created_on)
		SELECT dpt.device_profile_type_id, dpt.name, dpt.created_on
		FROM device_profiles_types dpt
		ON CONFLICT (sensor_profile_type_id) DO NOTHING;
	END IF;
END $$;

CREATE TABLE IF NOT EXISTS device_tags (
	name  		TEXT NOT NULL,
	created_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_tags PRIMARY KEY (name)
);

CREATE TABLE IF NOT EXISTS sensor_status (
	observed_at		timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	sensor_id		TEXT 	NOT NULL,
	battery_level 	NUMERIC NULL,
	rssi 			NUMERIC NULL,
	snr 			NUMERIC NULL,
	fq 				NUMERIC NULL,
	sf 				NUMERIC NULL,
	dr 				NUMERIC NULL,
	created_on  	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_sensor_status PRIMARY KEY (observed_at, sensor_id),
	CONSTRAINT fk_sensor_sensor_status FOREIGN KEY (sensor_id) REFERENCES sensors (sensor_id) ON DELETE CASCADE
);

DO $$
BEGIN
	IF to_regclass('public.device_status') IS NOT NULL THEN
		INSERT INTO sensor_status (observed_at, sensor_id, battery_level, rssi, snr, fq, sf, dr, created_on)
		SELECT ds.observed_at, d.sensor_id, ds.battery_level, ds.rssi, ds.snr, ds.fq, ds.sf, ds.dr, ds.created_on
		FROM device_status ds
		JOIN devices d ON d.device_id = ds.device_id
		WHERE d.sensor_id IS NOT NULL
		ON CONFLICT (observed_at, sensor_id) DO NOTHING;
	END IF;
END $$;

CREATE TABLE IF NOT EXISTS device_state (
	device_id	TEXT NOT NULL,
	online 		BOOLEAN NOT NULL DEFAULT FALSE,
	state 		NUMERIC NOT NULL DEFAULT -1,
	observed_at	timestamp with time zone NULL,
	created_on 	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_state PRIMARY KEY (device_id),
	CONSTRAINT fk_device_device_state FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS device_alarms (
	device_id	TEXT NOT NULL,
	type		TEXT NOT NULL,
	description	TEXT NULL,
	severity	NUMERIC NOT NULL DEFAULT 0,
	count 		NUMERIC NOT NULL DEFAULT 0,
	observed_at	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_alarms PRIMARY KEY (device_id, type),
	CONSTRAINT fk_device_alarms FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS device_metadata (
	device_id	TEXT NOT NULL,
	key			TEXT NOT NULL,
	v 			NUMERIC NULL,
	vs			TEXT NULL,
	vb			BOOLEAN NULL,

	created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_metadata PRIMARY KEY (device_id, key),
	CONSTRAINT fk_device_metadata FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
);

ALTER TABLE device_metadata DROP COLUMN IF EXISTS name;
ALTER TABLE device_metadata DROP COLUMN IF EXISTS description;

CREATE TABLE IF NOT EXISTS device_device_tags (
	device_id 	TEXT NOT NULL,
	name  		TEXT NOT NULL,
	created_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_device_tags PRIMARY KEY (device_id, name),
	CONSTRAINT fk_device_device_tags_device FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE,
	CONSTRAINT fk_device_device_tags_tags FOREIGN KEY (name) REFERENCES device_tags (name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sensor_profiles_sensor_profile_types (
	sensor_profile_id 		TEXT NOT NULL,
	sensor_profile_type_id	TEXT NOT NULL,
	created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_sensor_profiles_sensor_profile_types PRIMARY KEY (sensor_profile_id, sensor_profile_type_id),
	CONSTRAINT fk_sensor_profiles_sensor_profile_types_profile FOREIGN KEY (sensor_profile_id) REFERENCES sensor_profiles (sensor_profile_id) ON DELETE CASCADE,
	CONSTRAINT fk_sensor_profiles_sensor_profile_types_type FOREIGN KEY (sensor_profile_type_id) REFERENCES sensor_profile_types (sensor_profile_type_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS device_sensor_profile_types (
	device_id 				TEXT NOT NULL,
	sensor_profile_type_id	TEXT NOT NULL,
	created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT pk_device_sensor_profile_types PRIMARY KEY (device_id, sensor_profile_type_id),
	CONSTRAINT fk_device_sensor_profile_types_device FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE,
	CONSTRAINT fk_device_sensor_profile_types_type FOREIGN KEY (sensor_profile_type_id) REFERENCES sensor_profile_types (sensor_profile_type_id) ON DELETE CASCADE
);

DO $$
BEGIN
	IF to_regclass('public.device_profiles_device_profiles_types') IS NOT NULL THEN
		INSERT INTO sensor_profiles_sensor_profile_types (sensor_profile_id, sensor_profile_type_id, created_on)
		SELECT dpdpt.device_profile_id, dpdpt.device_profile_type_id, dpdpt.created_on
		FROM device_profiles_device_profiles_types dpdpt
		ON CONFLICT (sensor_profile_id, sensor_profile_type_id) DO NOTHING;
	END IF;
END $$;

DO $$
BEGIN
	IF to_regclass('public.device_device_profile_types') IS NOT NULL THEN
		INSERT INTO device_sensor_profile_types (device_id, sensor_profile_type_id, created_on)
		SELECT ddpt.device_id, ddpt.device_profile_type_id, ddpt.created_on
		FROM device_device_profile_types ddpt
		ON CONFLICT (device_id, sensor_profile_type_id) DO NOTHING;
	END IF;
END $$;

UPDATE devices SET sensor_id = NULL WHERE sensor_id = '';

INSERT INTO sensors (sensor_id)
SELECT DISTINCT sensor_id
FROM devices
WHERE sensor_id IS NOT NULL
ON CONFLICT (sensor_id) DO NOTHING;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = 'devices' AND column_name = 'device_profile'
	) THEN
		UPDATE sensors s
		SET sensor_profile = d.device_profile,
			modified_on = NOW()
		FROM devices d
		WHERE d.sensor_id = s.sensor_id
			AND d.device_profile IS NOT NULL
			AND (s.sensor_profile IS NULL OR s.sensor_profile = '');
	END IF;
END $$;

ALTER TABLE devices DROP COLUMN IF EXISTS device_profile;

UPDATE sensors s
SET sensor_profile = NULL
WHERE s.sensor_profile IS NOT NULL
	AND NOT EXISTS (
		SELECT 1
		FROM sensor_profiles sp
		WHERE sp.sensor_profile_id = s.sensor_profile
	);

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_devices_sensor'
	) THEN
		ALTER TABLE devices
			ADD CONSTRAINT fk_devices_sensor
			FOREIGN KEY (sensor_id) REFERENCES sensors (sensor_id) ON DELETE SET NULL;
	END IF;
END $$;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_sensors_sensor_profile'
	) THEN
		ALTER TABLE sensors
			ADD CONSTRAINT fk_sensors_sensor_profile
			FOREIGN KEY (sensor_profile) REFERENCES sensor_profiles (sensor_profile_id) ON DELETE SET NULL;
	END IF;
END $$;

DROP INDEX IF EXISTS uq_devices_sensor_not_deleted;
CREATE UNIQUE INDEX IF NOT EXISTS uq_devices_sensor_not_deleted ON devices(sensor_id) WHERE deleted = FALSE AND sensor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_device_state_device_id ON device_state(device_id);
CREATE INDEX IF NOT EXISTS idx_device_device_tags_name ON device_device_tags(name);
CREATE INDEX IF NOT EXISTS idx_sensor_status_sensor_id_observed_at ON sensor_status(sensor_id, observed_at DESC);
DROP INDEX IF EXISTS idx_device_device_profile_types_type;
CREATE INDEX IF NOT EXISTS idx_device_sensor_profile_types_type ON device_sensor_profile_types(sensor_profile_type_id);

DROP TABLE IF EXISTS device_profiles_device_profiles_types;
DROP TABLE IF EXISTS device_device_profile_types;
DROP TABLE IF EXISTS device_profiles_types;
DROP TABLE IF EXISTS device_profiles;
DROP TABLE IF EXISTS device_status;
