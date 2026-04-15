package query

type Alarms struct {
	AlarmType      string
	AllowedTenants []string
	ActiveOnly     bool
	Offset         *int
	Limit          *int
}
