package config

import (
	"time"
	"tron-tss/types"
)

var (
	SecretManagerSlaveConfigMap = map[int]types.SecretManagerSlaveConfig{
		1: {
			Host: "ws://localhost:8081/",
		},
		2: {
			Host: "ws://localhost:8082/",
		},
		3: {
			Host: "ws://localhost:8083/",
		},
		4: {
			Host: "ws://localhost:8084/",
		},
		5: {
			Host: "ws://localhost:8085/",
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
