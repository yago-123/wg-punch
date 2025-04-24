package errors

import (
	"errors"
	"fmt"
)

var (
	// Connector errors
	ErrBindingUDP      = errors.New("failed to bind UDP")
	ErrPubAddrRetrieve = errors.New("failed to get public address")
	ErrRegisterPeer    = errors.New("failed to register with rendezvous server")
	ErrWaitForPeer     = errors.New("failed to wait for remote peer")
	ErrPunchingNAT     = errors.New("failed to perform UDP hole punching")
	ErrConvertAllowed  = errors.New("failed to convert allowed IPs")
	ErrTunnelStart     = errors.New("failed to start wireguard tunnel")
)

func Wrap(step error, err error) error {
	return fmt.Errorf("%w: %w", step, err)
}
