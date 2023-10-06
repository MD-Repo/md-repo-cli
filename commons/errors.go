package commons

import "golang.org/x/xerrors"

var (
	InvalidTicketError           error = xerrors.Errorf("invalid ticket string")
	InvalidTokenError            error = xerrors.Errorf("invalid token")
	TokenNotProvidedError        error = xerrors.Errorf("token not provided")
	InvalidOrcIDError            error = xerrors.Errorf("invalid ORC-ID")
	SimulationNoNotMatchingError error = xerrors.Errorf("simulation number not match")
)
