package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

func s256(s []byte) []byte {
	h := sha256.New()
	h.Write(s)
	bs := h.Sum(nil)
	return bs
}

func loadKeygenTestFixtures(qty int, optionalStart ...int) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs, error) {
	keys := make([]keygen.LocalPartySaveData, 0, qty)
	start := 0

	if 0 < len(optionalStart) {
		start = optionalStart[0]
	}

	for i := start; i < qty; i++ {
		fixtureFilePath := makeTestFixtureFilePath(i)
		bz, err := os.ReadFile(fixtureFilePath)
		if err != nil {
			return nil, nil, err
		}
		var key keygen.LocalPartySaveData
		if err = json.Unmarshal(bz, &key); err != nil {
			return nil, nil, err
		}
		for _, kbxj := range key.BigXj {
			kbxj.SetCurve(tss.S256())
		}
		key.ECDSAPub.SetCurve(tss.S256())
		keys = append(keys, key)
	}

	partyIDs := make(tss.UnSortedPartyIDs, len(keys))
	for i, key := range keys {
		pMoniker := fmt.Sprintf("%d", i+start+1)
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, key.ShareID)
	}

	sortedPIDs := tss.SortPartyIDs(partyIDs)
	return keys, sortedPIDs, nil
}

func makeTestFixtureFilePath(partyIndex int) string {
	return fmt.Sprintf("%s/"+testFixtureFileFormat, testFixtureDirFormat, partyIndex)
}

func loadKeyDataFromFile(filename string) ([]keygen.LocalPartySaveData, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var keys []keygen.LocalPartySaveData
	if err := json.NewDecoder(f).Decode(&keys); err != nil {
		return nil, err
	}

	return keys, nil
}

func saveKeyDataToFile(filename string, keys []keygen.LocalPartySaveData) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(keys); err != nil {
		return err
	}

	return nil
}
