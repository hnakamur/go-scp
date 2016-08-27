package scp

type ProtocolError struct {
	msg   string
	fatal bool
}

func (e *ProtocolError) Error() string { return e.msg }
func (e *ProtocolError) Fatal() bool   { return e.fatal }
