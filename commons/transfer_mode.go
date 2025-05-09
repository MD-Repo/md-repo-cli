package commons

type TransferMode string

const (
	TransferModeICAT     TransferMode = "icat"
	TransferModeRedirect TransferMode = "redirect"
	TransferModeWebDAV   TransferMode = "webdav"
)

func (t TransferMode) Valid() bool {
	if t == TransferModeICAT || t == TransferModeRedirect || t == TransferModeWebDAV {
		return true
	}
	return false
}
