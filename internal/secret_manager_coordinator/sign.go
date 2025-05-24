package secret_manager_coordinator

type Sign struct {
}

func (s *Sign) Sign(address string, unsignedPayload []byte) ([]byte, error) {
	return nil, nil
}
