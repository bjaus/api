package api

// SelfValidator is implemented by request types that validate themselves.
type SelfValidator interface {
	Validate() error
}

// Validator validates any request.
type Validator interface {
	Validate(req any) error
}
