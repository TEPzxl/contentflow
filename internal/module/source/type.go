package source

const (
	TypeRSS   = "rss"
	TypeEmail = "email"
)

func IsValidType(t string) bool {
	switch t {
	case TypeRSS, TypeEmail:
		return true
	default:
		return false
	}
}
