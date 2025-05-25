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

type KeyGenRequest struct {
	connMap   sync.Map
	connMu    sync.Mutex
	partyIDs  tss.SortedPartyIDs
	genResult sync.Map
}

func (r *KeyGenRequest) GenerateTronAddress() (string, error) {
	r.partyIDs = utils.GeneratePartyIDs()
	var wg sync.WaitGroup

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
				case types.MsgTypeKeyGenCommunicate:
					err = r.handleIncomingCommunicateMsg(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenCommunicate:", err)
						continue
					}

				case types.MsgTypeKeyGenDone:
					wg.Done()

					err = r.handleIncomingDoneMsg(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenDone:", err)
					}
					break

				case types.MsgTypeKeyGenError:
					err = r.handleIncomingErrorMsg(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenError:", err)
					}
					break
				}
			}
		}(conn)
	}

	// start gen
	data, _ := json.Marshal(types.KeyGenStartMsg{
		Threshold: config.Threshold,
		PartyIDs:  r.partyIDs,
	})

	startMsg := types.Msg{
		RequestUUID: uuid.New().String(),
		Type:        types.MsgTypeKeyGenStart,
		Data:        data,
	}

	for i, conn := range r.connMap.Range {
		conn := conn.(*websocket.Conn)

		err := conn.WriteJSON(startMsg)
		if err != nil {
			log.Panicf("Fail to send start gen message to party %v: %v", i, err)
		}
	}

	// waiting for results
	log.Println("Waiting for keygen to be generated...")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var err error

	select {
	case <-done:
		log.Println("Keygen generated successfully!")
	case <-time.After(config.Timeout):
		log.Println("Timeout waiting for keygen. Abort!")
		err = errors.New("Timeout waiting for keygen. Abort!")
	}

	if err != nil {
		return "", err
	}

	// format result
	var tronAddress string
	err = nil

	r.genResult.Range(func(i, result any) bool {
		addr := result.(string)

		if tronAddress != "" && tronAddress != addr {
			err = fmt.Errorf("not all generated tron addresses are equal: %v vs %v", tronAddress, addr)
			return false
		}

		tronAddress = addr

		return true
	})

	if err != nil {
		return "", err
	}

	return tronAddress, nil
}

func (r *KeyGenRequest) handleIncomingCommunicateMsg(msgStruct types.Msg) error {
	var data types.KeyGenCommunicateMsg

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

func (r *KeyGenRequest) handleIncomingDoneMsg(msgStruct types.Msg) error {
	var data types.KeyGenDoneMsg

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	log.Printf("Receive generated tron address from %v: %v", data.From.Id, data.Address)

	r.genResult.Store(data.From.Id, data.Address)

	return nil
}

func (r *KeyGenRequest) handleIncomingErrorMsg(msgStruct types.Msg) error {
	var data types.KeyGenErrorMsg

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	log.Printf("Error key gen from party %v: %v", data.From.Id, data.Err)

	return nil
}

func (r *KeyGenRequest) routeMsg(to string, msg types.Msg) error {
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
