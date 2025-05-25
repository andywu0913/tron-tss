package secret_manager_coordinator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
	"tron-tss/config"
	"tron-tss/internal/utils"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SignRequest struct {
	connMap    sync.Map
	connMu     sync.Mutex
	partyIDs   tss.SortedPartyIDs
	signResult sync.Map
}

func (r *SignRequest) SignTx(address string, unsignedPayload []byte) (string, error) {
	r.partyIDs = utils.GeneratePartyIDs()
	var wg sync.WaitGroup

	// can be randomly picked
	r.partyIDs = r.partyIDs[:config.Threshold+1]

	// init conn
	for _, partyID := range r.partyIDs {
		config, ok := config.SecretManagerPartyConfigMap[partyID.Id]
		if !ok {
			return "", fmt.Errorf("Config not found for partyID %v", partyID.Id)
		}

		conn, _, err := websocket.DefaultDialer.Dial(
			fmt.Sprintf("ws://%v:%v/", config.Host, config.Port),
			nil,
		)
		if err != nil {
			return "", fmt.Errorf("Error connecting to secret manager party %v: %v", partyID.Id, err)
		}

		log.Printf("Connected to secret manager party %v", partyID.Id)

		r.connMap.Store(partyID.Id, conn)

		wg.Add(1)
		go func(conn *websocket.Conn) {
			defer conn.Close()

			for {
				var msgStruct types.Msg

				err := conn.ReadJSON(&msgStruct)
				if err != nil {
					log.Printf("Error reading message from party %v: %T: %v", partyID.Id, err, err)
					break
				}

				log.Printf("Get message from party %v", partyID.Id)

				switch msgStruct.Type {
				case types.MsgTypeSignCommunicate:
					err = r.handleIncomingCommunicateMsg(msgStruct)
					if err != nil {
						log.Println("Error handleSignCommunicate:", err)
						continue
					}

				case types.MsgTypeSignDone:
					wg.Done()

					err = r.handleIncomingDoneMsg(msgStruct)
					if err != nil {
						log.Println("Error handleSignDone:", err)
					}
					break

				case types.MsgTypeSignError:
					err = r.handleIncomingErrorMsg(msgStruct)
					if err != nil {
						log.Println("Error handleSignError:", err)
					}
					break
				}
			}
		}(conn)
	}

	// start sign
	txHash := utils.SHA256(unsignedPayload)
	log.Printf("Transaction hash to sign: %x", txHash)

	data, _ := json.Marshal(types.SignStartMsg{
		FromAddress: address,
		MessageHash: txHash,
		Threshold:   config.Threshold,
		PartyIDs:    r.partyIDs,
	})

	startMsg := types.Msg{
		RequestUUID: uuid.New().String(),
		Type:        types.MsgTypeSignStart,
		Data:        data,
	}

	for _, partyID := range r.partyIDs {
		conn, ok := r.connMap.Load(partyID.Id)
		if !ok {
			log.Panicf("Fail to get connection for party %v", partyID.Id)
		}

		err := conn.(*websocket.Conn).WriteJSON(startMsg)
		if err != nil {
			log.Panicf("Fail to send start sign message to party %v: %v", partyID.Id, err)
		}
	}

	// waiting for results
	log.Println("Waiting for signing to be completed...")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var err error

	select {
	case <-done:
		log.Println("Sign successfully!")
	case <-time.After(config.Timeout):
		log.Println("Timeout waiting for signing. Abort!")
		err = errors.New("Timeout waiting for signing. Abort!")
	}

	if err != nil {
		return "", err
	}

	// format result
	var signature string
	err = nil

	r.signResult.Range(func(i, result any) bool {
		sig := result.(string)

		if signature != "" && signature != sig {
			err = fmt.Errorf("not all signatures are equal: %v vs %v", signature, sig)
			return false
		}

		signature = sig

		return true
	})

	if err != nil {
		return "", err
	}

	return signature, nil
}

func (r *SignRequest) handleIncomingCommunicateMsg(msgStruct types.Msg) error {
	var data types.SignCommunicateMsg

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	if *data.IsBroadcast { // broadcast
		log.Printf("Receive broadcast message from %v to all", data.From.Id)

		for _, p := range r.partyIDs {
			if p.Index == data.From.Index {
				continue
			}

			log.Printf("Forwarding message to: %+v", p)
			err = r.routeMsg(p.Id, msgStruct)
			if err != nil {
				log.Panicf("Fail to send communicate message to party %v: %v", p.Id, err)
			}
		}
	} else { // point-to-point
		log.Printf("Receive direct message from %v to %v", data.From.Id, data.To[0].Id)

		if data.To[0].Index == data.From.Index {
			log.Panicf("party %d tried to send a message to itself (%d)", data.To[0].Index, data.From.Index)
		}

		log.Printf("Forwarding message to: %+v", data.To[0])
		err = r.routeMsg(data.To[0].Id, msgStruct)
		if err != nil {
			log.Panicf("Fail to send communicate message to party %v: %v", data.To[0].Id, err)
		}
	}

	return nil
}

func (r *SignRequest) handleIncomingDoneMsg(msgStruct types.Msg) error {
	var data types.SignDoneMsg

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	log.Printf("Receive signature from %v: %v", data.From.Id, data.Signature)

	r.signResult.Store(data.From.Id, data.Signature)

	return nil
}

func (r *SignRequest) handleIncomingErrorMsg(msgStruct types.Msg) error {
	var data types.SignErrorMsg

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	log.Printf("Error sign from party %v: %v", data.From.Id, data.Err)

	return nil
}

func (r *SignRequest) routeMsg(to string, msg types.Msg) error {
	conn, ok := r.connMap.Load(to)
	if !ok {
		return fmt.Errorf("fail to load connMap: %v", to)
	}

	r.connMu.Lock()
	defer r.connMu.Unlock()

	err := conn.(*websocket.Conn).WriteJSON(msg)
	if err != nil {
		return err
	}

	return nil
}
