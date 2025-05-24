package types

import (
	"encoding/json"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

type SecretManagerSlaveConfig struct {
	Host string
}

const (
	MsgTypeGenKeyStart        = 10
	MsgTypeGenKeyCommunicate  = 11
	MsgTypeSignKeyStart       = 20
	MsgTypeSignKeyCommunicate = 21
)

type Msg struct {
	RequestUUID string          `json:"request_uuid"`
	Type        int             `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type MsgGenKeyStart struct {
	Threshold int                `json:"threshold"`
	PartyIDs  tss.SortedPartyIDs `json:"party_ids"`
}

type MsgGenKeyCommunicate struct {
	Err         *string        `json:"error"`
	Msg         *string        `json:"msg"`
	From        *tss.PartyID   `json:"get_from"`
	To          []*tss.PartyID `json:"get_to"`
	IsBroadcast *bool          `json:"is_broadcast"`

	errCh *tss.Error
	outCh tss.Message
	endCh *keygen.LocalPartySaveData
}

type RequestChannel struct {
	ErrCh chan *tss.Error
	OutCh chan tss.Message
	EndCh chan *keygen.LocalPartySaveData
	Party tss.Party
}
