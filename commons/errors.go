package commons

import "golang.org/x/xerrors"

var (
	InvalidTicketError           error = xerrors.Errorf("invalid ticket string")
	InvalidTokenError            error = xerrors.Errorf("invalid token")
	TokenNotProvidedError        error = xerrors.Errorf("token not provided")
	TicketNotReadyError          error = xerrors.Errorf("ticket not ready")
	InvalidOrcIDError            error = xerrors.Errorf("invalid ORC-ID")
	SimulationNoNotMatchingError error = xerrors.Errorf("simulation number not match")
	InvalidSubmitMetadataError   error = xerrors.Errorf("invalid submit metadata")
)
