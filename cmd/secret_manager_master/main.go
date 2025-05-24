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
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

func main() {
	gen()
}

func gen() {
	var connMap sync.Map
	// var endCounter int32
	done := make(chan struct{}, 1)

	// init conn
	for i, config := range config.SecretManagerSlaveConfigMap {
		conn, _, err := websocket.DefaultDialer.Dial(config.Host, nil)
		if err != nil {
			log.Printf("Error connecting to slave %v: %v", i, err)
			continue
		}

		log.Printf("Connected to secret manager slave %v", i)

		connMap.Store(strconv.Itoa(i), conn)

		go func(conn *websocket.Conn) {
			for {
				var msgStruct types.Msg

				err := conn.ReadJSON(&msgStruct)
				if err != nil {
					log.Panicf("Error reading message from slave %v: %v", i, err)
				}
				// log.Printf("Message from slave %v: %+v", i, msgStruct)
				log.Printf("Get message from slave %v", i)

				switch msgStruct.Type {
				case types.MsgTypeGenKeyCommunicate:
					var data types.MsgGenKeyCommunicate
					err := json.Unmarshal(msgStruct.Data, &data)
					if err != nil {
						log.Println("Error reading message:", err)
						continue
					}

					if data.Err != nil {
						log.Panicf("get error data: %v", data.Err)
					}

					// TODO: route

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
								log.Panicf("Fail to send communicate message to slave %v: %v", i, err)
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
							log.Panicf("Fail to send communicate message to slave %v: %v", i, err)
						}
					}
				}

			}
		}(conn)
	}

	// start gen
	data, _ := json.Marshal(types.MsgGenKeyStart{
		Threshold: config.Threshold,
		PartyIDs:  generatePartyIDs(),
	})

	startMsg := types.Msg{
		RequestUUID: "1",
		Type:        types.MsgTypeGenKeyStart,
		Data:        data,
	}

	for i, conn := range connMap.Range {
		conn := conn.(*websocket.Conn)

		err := conn.WriteJSON(startMsg)
		if err != nil {
			log.Panicf("Fail to send start gen message to slave %v: %v", i, err)
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
	partyIDs := make([]*tss.PartyID, 0, len(config.SecretManagerSlaveConfigMap))

	for i, _ := range config.SecretManagerSlaveConfigMap {
		key := big.NewInt(int64(i))
		// partyIDs[i] = tss.NewPartyID(fmt.Sprintf("%d", i), fmt.Sprintf("P%d", i), key)
		partyIDs = append(partyIDs, tss.NewPartyID(fmt.Sprintf("%d", i), fmt.Sprintf("P%d", i), key))
	}

	return tss.SortPartyIDs(partyIDs)
}

var wsMu sync.Mutex

func routeMsg(conn *websocket.Conn, msg types.Msg) error {
	wsMu.Lock()
	defer wsMu.Unlock()

	err := conn.WriteJSON(msg)
	if err != nil {
		return err
	}

	return nil
}
