package query

type Sensors struct {
	Offset      *int
	Limit       *int
	Assigned    *bool
	HasProfile  *bool
	ProfileName string
	Types       []string
	Search      string
}
