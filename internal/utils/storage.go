package utils

import (
	"encoding/json"
	"os"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
)

func StoreLocalPartyData(partyID string, dirPath string, filename string, saveData *keygen.LocalPartySaveData) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return err
		}
	}

	fullPath := dirPath + string(os.PathSeparator) + filename

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(saveData); err != nil {
		return err
	}

	return nil
}

func LoadLocalPartyData(partyID string, dirPath string, filename string) (*keygen.LocalPartySaveData, error) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return nil, err
		}
	}

	fullPath := dirPath + string(os.PathSeparator) + filename

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var saveData keygen.LocalPartySaveData
	if err := json.NewDecoder(f).Decode(&saveData); err != nil {
		return nil, err
	}

	return &saveData, nil
}
