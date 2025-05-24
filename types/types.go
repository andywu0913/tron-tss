package types

import (
	"encoding/json"

	"github.com/bnb-chain/tss-lib/v2/crypto"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

type SecretManagerSlaveConfig struct {
	Host string
}

const (
	MsgTypeKeyGenStart        = 10
	MsgTypeKeyGenCommunicate  = 11
	MsgTypeKeyGenDone         = 12
	MsgTypeKeyGenError        = 13
	MsgTypeSignKeyStart       = 20
	MsgTypeSignKeyCommunicate = 21
	MsgTypeSignKeyDone        = 22
	MsgTypeSignKeyError       = 23
)

type Msg struct {
	RequestUUID string          `json:"request_uuid"`
	Type        int             `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type MsgKeyGenStart struct {
	Threshold int                `json:"threshold"`
	PartyIDs  tss.SortedPartyIDs `json:"party_ids"`
}

type MsgKeyGenCommunicate struct {
	From        *tss.PartyID   `json:"from"`
	To          []*tss.PartyID `json:"to"`
	Msg         *string        `json:"msg"`
	IsBroadcast *bool          `json:"is_broadcast"`
}

type MsgKeyGenDone struct {
	From     *tss.PartyID    `json:"from"`
	ECDSAPub *crypto.ECPoint `json:"ecdsa_pub"`
}

type MsgKeyGenError struct {
	From *tss.PartyID `json:"from"`
	Err  *string      `json:"error"`
}

type RequestState struct {
	ErrCh chan *tss.Error
	OutCh chan tss.Message
	EndCh chan *keygen.LocalPartySaveData
	Party tss.Party
}
