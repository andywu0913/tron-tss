package tss

import (
	"log"
	"strconv"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

func GenStart(id int, threshold int, partyIDs tss.SortedPartyIDs, errCh chan *tss.Error, outCh chan tss.Message, endCh chan *keygen.LocalPartySaveData) tss.Party {
	peerCtx := tss.NewPeerContext(partyIDs)

	for _, partyID := range partyIDs {
		if partyID.Id != strconv.Itoa(id) {
			continue
		}

		params := tss.NewParameters(tss.S256(), peerCtx, partyID, len(partyIDs), threshold)

		var p tss.Party
		// if i < len(fixtures) {
		// 	p = keygen.NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams)
		// } else {
		// 	p = keygen.NewLocalParty(params, outCh, endCh)
		// }
		p = keygen.NewLocalParty(params, outCh, endCh)

		log.Printf("Starting keygen party %s", partyID.Id)
		go func(p tss.Party) {
			if err := p.Start(); err != nil {
				errCh <- err
				log.Printf("Error starting keygen party %s: %v", p.PartyID().Id, err)

			}
		}(p)

		return p
	}

	return nil
}
