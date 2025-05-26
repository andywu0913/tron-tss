package config

import (
	"fmt"
	"time"
	"tron-tss/types"
)

const (
	TronAPIBaseUrl    = "https://nile.trongrid.io"
	CreateTxEndpoint  = "/wallet/createtransaction"
	BroadcastEndpoint = "/wallet/broadcasttransaction"
)

const (
	Parties   = 5
	Threshold = 2
	Timeout   = 3 * 60 * time.Second
)

const (
	PARTY_1 = "1"
	PARTY_2 = "2"
	PARTY_3 = "3"
	PARTY_4 = "4"
	PARTY_5 = "5"
)

var (
	SecretManagerPartyConfigMap = map[string]types.SecretManagerPartyConfig{
		PARTY_1: {
			Host:        "localhost",
			Port:        8081,
			StoragePath: fmt.Sprintf("./data/%v/", PARTY_1),
		},
		PARTY_2: {
			Host:        "localhost",
			Port:        8082,
			StoragePath: fmt.Sprintf("./data/%v/", PARTY_2),
		},
		PARTY_3: {
			Host:        "localhost",
			Port:        8083,
			StoragePath: fmt.Sprintf("./data/%v/", PARTY_3),
		},
		PARTY_4: {
			Host:        "localhost",
			Port:        8084,
			StoragePath: fmt.Sprintf("./data/%v/", PARTY_4),
		},
		PARTY_5: {
			Host:        "localhost",
			Port:        8085,
			StoragePath: fmt.Sprintf("./data/%v/", PARTY_5),
		},
	}
)
