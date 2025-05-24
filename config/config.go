package config

import (
	"fmt"
	"time"
	"tron-tss/types"
)

const (
	PARTY_1 = 1
	PARTY_2 = 2
	PARTY_3 = 3
	PARTY_4 = 4
	PARTY_5 = 5
)

var (
	SecretManagerPartyConfigMap = map[int]types.SecretManagerPartyConfig{
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

const (
	TestFixtureDirFormat  = "/Users/user/Documents/test_golang/mpc_final/ecdsa_fixtures"
	TestFixtureFileFormat = "keygen_data_%d.json"
	KeygenFile            = "keygen2.json"
	Parties               = 5
	Threshold             = 2
	Timeout               = 3 * 60 * time.Second
)
