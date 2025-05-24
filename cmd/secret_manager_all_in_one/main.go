package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync/atomic"
	"time"
	"tron-tss/internal/utils"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	testFixtureDirFormat  = "/Users/user/Documents/test_golang/mpc_final/ecdsa_fixtures"
	testFixtureFileFormat = "keygen_data_%d.json"
	keygenFile            = "keygen2.json"
	parties               = 5
	threshold             = 2
	timeout               = 3 * 60 * time.Second
)

const (
	ApiBaseUrl        = "https://nile.trongrid.io"
	CreateTxEndpoint  = "/wallet/createtransaction"
	BroadcastEndpoint = "/wallet/broadcasttransaction"

	// toAddress = "TSe1MQvi6TAFz1h1YLYvVfiyiBkQwQs34v"
	toAddress = "TMhvZyXm3NgQfhR59gRZwihwjxVSEb9ahH"
	// toAddress = "TUvcHSxSYCGXRiTPprYU9aEkHfXdVCnTnA"
	amountTRX = 1
)

func main() {
	partyIDs := generatePartyIDs(parties)

	// try to load key data from disk
	keyData, err := loadKeyDataFromFile(keygenFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Panicf("Failed to load key data: %v", err)
		}

		log.Println("Key data not found, generating new key data...")

		keyData, err = gen(parties, threshold, partyIDs)
		if err != nil {
			log.Panicf("Failed to generate key data: %v", err)
		}
		saveKeyDataToFile(keygenFile, keyData)
	}

	pubKey := keyData[0].ECDSAPub

	// restore tron address from public key
	tronAddress, err := utils.GenerateTronAddress(&ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     pubKey.X(),
		Y:     pubKey.Y(),
	})
	if err != nil {
		log.Panicf("Failed to generate tron address: %v", err)
	}
	log.Println("Tron Address:", tronAddress)

	// create transaction
	transaction, err := createTransferTRXTransaction(tronAddress, toAddress, amountTRX*1_000_000)
	if err != nil {
		log.Panicf("Failed to create transaction: %v", err)
	}

	rawDataHexBytes, err := hex.DecodeString(transaction.RawDataHex)
	if err != nil {
		log.Panicf("Failed to decode raw data hex: %v", err)
	}
	txHash := utils.SHA256(rawDataHexBytes)
	log.Printf("Transaction hash to sign: %x", txHash)

	// sign transaction
	sig, err := sign(threshold, partyIDs, keyData, txHash)
	if err != nil {
		log.Panicf("Failed to sign transaction: %v", err)
	}

	// broadcast transaction
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)

	canonicalSig := formatCanonicalSignature(r, s)
	log.Printf("Canonical signature (r||s): %x", canonicalSig)

	var x btcec.FieldVal
	var y btcec.FieldVal

	x.SetByteSlice(pubKey.X().Bytes())
	y.SetByteSlice(pubKey.Y().Bytes())

	signature, err := findValidRecoveryID(txHash[:], canonicalSig, btcec.NewPublicKey(&x, &y))
	if err != nil {
		log.Panicf("Failed to find valid recovery ID: %v", err)
	}

	transaction.Signature = []string{hex.EncodeToString(signature)}

	response, err := broadcastTransaction(transaction)
	if err != nil {
		log.Panicf("Failed to broadcast transaction: %v", err)
	}

	log.Printf("Transaction broadcasted! Success: %v, TxID: %s", response.Result, response.TxID)
	log.Printf("Full response: %+v", response)
}

func gen(participants, threshold int, partyIDs tss.SortedPartyIDs) (keys []keygen.LocalPartySaveData, err error) {
	keys = make([]keygen.LocalPartySaveData, participants)

	fixtures, _, err := loadKeygenTestFixtures(participants)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Panicf("Failed to load test fixtures: %v", err)
		}

		log.Printf("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}

	peerCtx := tss.NewPeerContext(partyIDs)
	parties := make([]*keygen.LocalParty, 0, len(partyIDs))

	errCh := make(chan *tss.Error, len(partyIDs))
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan *keygen.LocalPartySaveData, len(partyIDs))

	log.Println("Creating keygen parties...")

	for i, partyID := range partyIDs {
		log.Printf("Creating keygen party %s", partyID.Id)

		params := tss.NewParameters(tss.S256(), peerCtx, partyID, len(partyIDs), threshold)

		var p tss.Party
		if i < len(fixtures) {
			p = keygen.NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams)
		} else {
			p = keygen.NewLocalParty(params, outCh, endCh)
		}
		parties = append(parties, p.(*keygen.LocalParty))

		log.Printf("Starting keygen party %s", partyID.Id)
		go func(p tss.Party) {
			if err := p.Start(); err != nil {
				errCh <- err
				log.Printf("Error starting keygen party %s: %v", p.PartyID().Id, err)
			}
		}(p)
	}

	var endCounter int32
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case err := <-errCh:
				log.Printf("Error: %s", err)
				return

			case msg := <-outCh:
				dest := msg.GetTo()

				if dest == nil { // broadcast
					log.Printf("Broadcast message from %v to all", msg.GetFrom().Id)

					for _, p := range parties {
						if p.PartyID().Index == msg.GetFrom().Index {
							continue
						}
						go sharedPartyUpdater(p, msg, errCh)
					}
				} else { // point-to-point
					log.Printf("Message from %v to %v", msg.GetFrom().Id, dest)

					if dest[0].Index == msg.GetFrom().Index {
						log.Panicf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					}
					go sharedPartyUpdater(parties[dest[0].Index], msg, errCh)
				}

			case saveData := <-endCh:
				idx, err := saveData.OriginalIndex()
				if err != nil {
					log.Panicf("Error getting original index: %s", err)
				}

				keys[idx] = *saveData
				log.Printf("Participant %d done", idx)

				atomic.AddInt32(&endCounter, 1)
				if atomic.LoadInt32(&endCounter) == int32(len(partyIDs)) {
					log.Printf("Done. Received save data from %d participants", endCounter)
					done <- struct{}{}
					return
				}
			}
		}
	}()

	log.Println("Waiting for keygen to be generated...")

	select {
	case <-done:
		log.Println("Keygen generated successfully!")
	case <-time.After(timeout):
		log.Println("Timeout waiting for keygen! Exiting...")
		close(done)
		return nil, fmt.Errorf("timeout waiting for keygen")
	}

	return keys, nil
}

func sign(threshold int, partyIDs tss.SortedPartyIDs, keys []keygen.LocalPartySaveData, messageHash []byte) (*common.SignatureData, error) {
	msgHashInt := new(big.Int).SetBytes(messageHash[:])

	partyIDs = partyIDs[:threshold+1]
	keys = keys[:threshold+1]

	peerCtx := tss.NewPeerContext(partyIDs)
	parties := make([]*signing.LocalParty, 0, len(partyIDs))

	errCh := make(chan *tss.Error, len(partyIDs))
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan *common.SignatureData, len(partyIDs))

	log.Println("Creating signing parties...")

	for i, partyID := range partyIDs {
		log.Printf("Creating signing party %s", partyID.Id)

		params := tss.NewParameters(tss.S256(), peerCtx, partyID, len(partyIDs), threshold)

		signParty := signing.NewLocalParty(msgHashInt, params, keys[i], outCh, endCh)
		parties = append(parties, signParty.(*signing.LocalParty))

		log.Printf("Starting signing party %s", partyID.Id)
		go func(p tss.Party) {
			if err := p.Start(); err != nil {
				errCh <- err
				log.Printf("Error starting signing party %s: %v", p.PartyID().Id, err)
			}
		}(signParty)
	}

	var endCounter int32
	done := make(chan common.SignatureData, 1)
	go func() {
		for {
			select {
			case err := <-errCh:
				log.Printf("Error: %s", err)
				return

			case msg := <-outCh:
				dest := msg.GetTo()

				if dest == nil { // broadcast
					log.Printf("Broadcast message from %v to all", msg.GetFrom().Id)

					for _, p := range parties {
						if p.PartyID().Index == msg.GetFrom().Index {
							continue
						}
						go sharedPartyUpdater(p, msg, errCh)
					}
				} else { // point-to-point
					log.Printf("Message from %v to %v", msg.GetFrom().Id, dest)

					if dest[0].Index == msg.GetFrom().Index {
						log.Panicf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					}
					go sharedPartyUpdater(parties[dest[0].Index], msg, errCh)
				}

			case sig := <-endCh:
				log.Printf("received signature data M: %x, R: %x, S: %x", sig.M, sig.R, sig.S)

				atomic.AddInt32(&endCounter, 1)
				if atomic.LoadInt32(&endCounter) == int32(len(partyIDs)) {
					log.Printf("Done. Received signature data from %d participants", endCounter)
					log.Printf("Signature data: %s", hex.EncodeToString(sig.GetSignature()))
					done <- *sig
					return
				}
			}
		}
	}()

	log.Println("Waiting for signature to be generated...")

	var sig common.SignatureData
	select {
	case sig = <-done:
		log.Println("Signature received successfully!")
	case <-time.After(timeout):
		log.Println("Timeout waiting for signature! Exiting...")
		close(done)
		return nil, fmt.Errorf("timeout waiting for signature")
	}

	return &sig, nil
}

func generatePartyIDs(n int) tss.SortedPartyIDs {
	partyIDs := make([]*tss.PartyID, n)

	for i := 0; i < n; i++ {
		key := big.NewInt(int64(i + 1))
		partyIDs[i] = tss.NewPartyID(fmt.Sprintf("%d", i), fmt.Sprintf("P%d", i), key)
	}

	return tss.SortPartyIDs(partyIDs)
}
func sharedPartyUpdater(party tss.Party, msg tss.Message, errCh chan<- *tss.Error) {
	// do not send a message from this party back to itself
	// if party.PartyID() == msg.GetFrom() {
	// 	return
	// }

	bz, _, err := msg.WireBytes()
	if err != nil {
		errCh <- party.WrapError(err)
		return
	}

	pMsg, err := tss.ParseWireMessage(bz, msg.GetFrom(), msg.IsBroadcast())
	if err != nil {
		errCh <- party.WrapError(err)
		return
	}

	if _, err := party.Update(pMsg); err != nil {
		errCh <- err
	}
}
