package utils

import (
	"fmt"
	"math/big"
	"strconv"
	"tron-tss/config"

	"github.com/bnb-chain/tss-lib/v2/tss"
)

func GeneratePartyIDs() tss.SortedPartyIDs {
	partyIDs := make([]*tss.PartyID, 0, len(config.SecretManagerPartyConfigMap))

	for i := range config.SecretManagerPartyConfigMap {
		i, _ := strconv.Atoi(i)

		partyIDs = append(partyIDs, tss.NewPartyID(
			fmt.Sprintf("%d", i),
			fmt.Sprintf("P%d", i),
			big.NewInt(int64(i)),
		))
	}

	return tss.SortPartyIDs(partyIDs)
}
