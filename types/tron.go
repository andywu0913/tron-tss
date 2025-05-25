package types

type Transaction struct {
	Visible    bool     `json:"visible"`
	TxID       string   `json:"txID,omitempty"`
	Signature  []string `json:"signature,omitempty"`
	RawDataHex string   `json:"raw_data_hex"`
	RawData    struct {
		RefBlockBytes string `json:"ref_block_bytes"`
		RefBlockHash  string `json:"ref_block_hash"`
		Expiration    int64  `json:"expiration"`
		Timestamp     int64  `json:"timestamp"`
		Contract      []struct {
			Type      string `json:"type"`
			Parameter struct {
				TypeUrl string `json:"type_url"`
				Value   struct {
					OwnerAddress string `json:"owner_address"`
					ToAddress    string `json:"to_address"`
					Amount       int64  `json:"amount"`
				} `json:"value"`
			} `json:"parameter"`
		} `json:"contract"`
	} `json:"raw_data"`
}

type BroadcastResponse struct {
	Result  bool   `json:"result"`
	TxID    string `json:"txid,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
