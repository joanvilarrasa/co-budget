package lib

type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

type SortConfig struct {
	Key       string
	Direction SortDirection
}
