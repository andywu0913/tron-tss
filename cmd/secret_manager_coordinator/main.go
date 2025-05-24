package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"sync"
	"time"
	"tron-tss/config"
	"tron-tss/internal/utils"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var connMap sync.Map

func main() {
	gen()
}

func gen() {
	done := make(chan struct{}, 1)

	// init conn
	for i, config := range config.SecretManagerPartyConfigMap {
		conn, _, err := websocket.DefaultDialer.Dial(config.Host, nil)
		if err != nil {
			log.Printf("Error connecting to party %v: %v", i, err)
			continue
		}

		log.Printf("Connected to secret manager party %v", i)

		connMap.Store(strconv.Itoa(i), conn)

		go func(conn *websocket.Conn) {
			for {
				var msgStruct types.Msg

				err := conn.ReadJSON(&msgStruct)
				if err != nil {
					log.Panicf("Error reading message from party %v: %v", i, err)
				}
				// log.Printf("Message from party %v: %+v", i, msgStruct)
				log.Printf("Get message from party %v", i)

				switch msgStruct.Type {
				case types.MsgTypeKeyGenCommunicate:
					err = handleKeyGenCommunicate(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenCommunicate:", err)
						continue
					}

				case types.MsgTypeKeyGenDone:
					err = handleKeyGenDone(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenDone:", err)
						return
					}

				case types.MsgTypeKeyGenError:
					err = handleKeyGenError(msgStruct)
					if err != nil {
						log.Println("Error handleKeyGenError:", err)
						return
					}
				}
			}
		}(conn)
	}

	// start gen
	data, _ := json.Marshal(types.MsgKeyGenStart{
		Threshold: config.Threshold,
		PartyIDs:  generatePartyIDs(),
	})

	startMsg := types.Msg{
		RequestUUID: uuid.New().String(),
		Type:        types.MsgTypeKeyGenStart,
		Data:        data,
	}

	for i, conn := range connMap.Range {
		conn := conn.(*websocket.Conn)

		err := conn.WriteJSON(startMsg)
		if err != nil {
			log.Panicf("Fail to send start gen message to party %v: %v", i, err)
		}
	}

	log.Println("Waiting for keygen to be generated...")

	select {
	case <-done:
		log.Println("Keygen generated successfully!")
	case <-time.After(config.Timeout):
		log.Println("Timeout waiting for keygen! Exiting...")
		close(done)
	}
}

func generatePartyIDs() tss.SortedPartyIDs {
	partyIDs := make([]*tss.PartyID, 0, len(config.SecretManagerPartyConfigMap))

	for i, _ := range config.SecretManagerPartyConfigMap {
		key := big.NewInt(int64(i))
		// partyIDs[i] = tss.NewPartyID(fmt.Sprintf("%d", i), fmt.Sprintf("P%d", i), key)
		partyIDs = append(partyIDs, tss.NewPartyID(fmt.Sprintf("%d", i), fmt.Sprintf("P%d", i), key))
	}

	return tss.SortPartyIDs(partyIDs)
}

func handleKeyGenCommunicate(msgStruct types.Msg) error {
	var data types.MsgKeyGenCommunicate

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	if *data.IsBroadcast { // broadcast
		log.Printf("Broadcast message from %v to all", data.From.Id)

		for _, p := range generatePartyIDs() {
			if p.Index == data.From.Index {
				continue
			}

			conn, ok := connMap.Load(p.Id)
			if !ok {
				log.Panicf("fail to load connMap first: %v", p.Id)
			}

			log.Printf("Sending message to: %+v", p)
			err = routeMsg(conn.(*websocket.Conn), msgStruct)
			if err != nil {
				log.Panicf("Fail to send communicate message to party %v: %v", p.Id, err)
			}
		}
	} else { // point-to-point
		log.Printf("Message from %v to %v", data.From.Id, data.To[0].Id)

		if data.To[0].Index == data.From.Index {
			log.Panicf("party %d tried to send a message to itself (%d)", data.To[0].Index, data.From.Index)
		}

		conn, ok := connMap.Load(data.To[0].Id)
		if !ok {
			log.Panicf("fail to load connMap second: %v", data.To[0].Id)
		}

		log.Printf("Sending message to: %+v", data.To[0])
		err = routeMsg(conn.(*websocket.Conn), msgStruct)
		if err != nil {
			log.Panicf("Fail to send communicate message to party %v: %v", data.To[0].Id, err)
		}
	}

	return nil
}

func handleKeyGenDone(msgStruct types.Msg) error {
	var data types.MsgKeyGenDone

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	tronAddress, err := utils.GenerateTronAddress(data.ECDSAPub.ToECDSAPubKey())
	if err != nil {
		return fmt.Errorf("Failed to generate tron address: %w", err)
	}

	log.Println("Tron Address:", tronAddress)

	return nil
}

func handleKeyGenError(msgStruct types.Msg) error {
	var data types.MsgKeyGenError

	err := json.Unmarshal(msgStruct.Data, &data)
	if err != nil {
		return fmt.Errorf("Error unmarshal message: %w", err)
	}

	log.Printf("Error key gen from party %v: %v", data.From.Id, data.Err)

	return nil
}

var routeMsgMu sync.Mutex

func routeMsg(conn *websocket.Conn, msg types.Msg) error {
	routeMsgMu.Lock()
	defer routeMsgMu.Unlock()

	err := conn.WriteJSON(msg)
	if err != nil {
		return err
	}

	return nil
}
