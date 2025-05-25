package types

import (
	"encoding/json"

	"github.com/bnb-chain/tss-lib/v2/tss"
)

type SecretManagerPartyConfig struct {
	Host        string
	Port        int
	StoragePath string
}

const (
	MsgTypeKeyGenStart       = 10
	MsgTypeKeyGenCommunicate = 11
	MsgTypeKeyGenDone        = 12
	MsgTypeKeyGenError       = 13
	MsgTypeSignStart         = 20
	MsgTypeSignCommunicate   = 21
	MsgTypeSignDone          = 22
	MsgTypeSignError         = 23
)

type Msg struct {
	RequestUUID string          `json:"request_uuid"`
	Type        int             `json:"type"`
	Data        json.RawMessage `json:"data"`
}

// coordinator -> party
type KeyGenStartMsg struct {
	Threshold int                `json:"threshold"`
	PartyIDs  tss.SortedPartyIDs `json:"party_ids"`
}

// party -> coordinator -> party
type KeyGenCommunicateMsg struct {
	From        *tss.PartyID   `json:"from"`
	To          []*tss.PartyID `json:"to"`
	Msg         *string        `json:"msg"`
	IsBroadcast *bool          `json:"is_broadcast"`
}

// party -> coordinator
type KeyGenDoneMsg struct {
	From    *tss.PartyID `json:"from"`
	Address string       `json:"address"`
}

// party -> coordinator
type KeyGenErrorMsg struct {
	From *tss.PartyID `json:"from"`
	Err  *string      `json:"error"`
}

type SignStartMsg struct {
	FromAddress string             `json:"from_address"`
	MessageHash []byte             `json:"msg_hash"`
	Threshold   int                `json:"threshold"`
	PartyIDs    tss.SortedPartyIDs `json:"party_ids"`
}

type SignCommunicateMsg struct {
	From        *tss.PartyID   `json:"from"`
	To          []*tss.PartyID `json:"to"`
	Msg         *string        `json:"msg"`
	IsBroadcast *bool          `json:"is_broadcast"`
}

type SignDoneMsg struct {
	From      *tss.PartyID `json:"from"`
	Signature string       `json:"signature"`
}

type SignErrorMsg struct {
	From *tss.PartyID `json:"from"`
	Err  *string      `json:"error"`
}
