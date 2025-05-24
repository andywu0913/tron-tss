package secret_manager_party

import (
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

type SignHandler struct {
	party tss.Party
	errCh chan *tss.Error
	outCh chan tss.Message
	endCh chan *keygen.LocalPartySaveData
}

func (h *SignHandler) handleIncomingStartMsg(partyID int, requestUUID string, conn *websocket.Conn, threshold int, partyIDs tss.SortedPartyIDs) {
}
func (h *SignHandler) handleIncomingCommunicateMsg(data types.KeyGenCommunicateMsg) {
}
