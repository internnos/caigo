package caigo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
)

type TransactionStatus struct {
	TxStatus  string `json:"tx_status"`
	BlockHash string `json:"block_hash"`
}

type StarknetTransaction struct {
	TransactionIndex int           `json:"transaction_index"`
	BlockNumber      int           `json:"block_number"`
	Transaction      JSTransaction `json:"transaction"`
	BlockHash        string        `json:"block_hash"`
	Status           string        `json:"status"`
}

// Starknet transaction composition
type Transaction struct {
	Calldata           []*big.Int `json:"calldata"`
	ContractAddress    *big.Int   `json:"contract_address"`
	EntryPointSelector *big.Int   `json:"entry_point_selector"`
	EntryPointType     string     `json:"entry_point_type"`
	Signature          []*big.Int `json:"signature"`
	TransactionHash    *big.Int   `json:"transaction_hash"`
	Type               string     `json:"type"`
	Nonce              *big.Int   `json:"nonce,omitempty"`
}

type StarknetGateway struct {
	Base    string   `json:"base"`
	Feeder  string   `json:"feeder"`
	Gateway string   `json:"gateway"`
	ChainId *big.Int `json:"chainId"`
}

type Block struct {
	BlockHash           string               `json:"block_hash"`
	ParentBlockHash     string               `json:"parent_block_hash"`
	BlockNumber         int                  `json:"block_number"`
	StateRoot           string               `json:"state_root"`
	Status              string               `json:"status"`
	Transactions        []JSTransaction      `json:"transactions"`
	Timestamp           int                  `json:"timestamp"`
	TransactionReceipts []TransactionReceipt `json:"transaction_receipts"`
}

type TransactionReceipt struct {
	Status                string `json:"status"`
	BlockHash             string `json:"block_hash"`
	BlockNumber           int    `json:"block_number"`
	TransactionIndex      int    `json:"transaction_index"`
	TransactionHash       string `json:"transaction_hash"`
	L1ToL2ConsumedMessage struct {
		FromAddress string   `json:"from_address"`
		ToAddress   string   `json:"to_address"`
		Selector    string   `json:"selector"`
		Payload     []string `json:"payload"`
	} `json:"l1_to_l2_consumed_message"`
	L2ToL1Messages     []interface{} `json:"l2_to_l1_messages"`
	Events             []interface{} `json:"events"`
	ExecutionResources struct {
		NSteps                 int `json:"n_steps"`
		BuiltinInstanceCounter struct {
			PedersenBuiltin   int `json:"pedersen_builtin"`
			RangeCheckBuiltin int `json:"range_check_builtin"`
			BitwiseBuiltin    int `json:"bitwise_builtin"`
			OutputBuiltin     int `json:"output_builtin"`
			EcdsaBuiltin      int `json:"ecdsa_builtin"`
			EcOpBuiltin       int `json:"ec_op_builtin"`
		} `json:"builtin_instance_counter"`
		NMemoryHoles int `json:"n_memory_holes"`
	} `json:"execution_resources"`
}

type ContractCode struct {
	Bytecode []string `json:"bytecode"`
	Abi      []ABI    `json:"abi"`
}

type ABI struct {
	Members []struct {
		Name   string `json:"name"`
		Offset int    `json:"offset"`
		Type   string `json:"type"`
	} `json:"members,omitempty"`
	Name   string `json:"name"`
	Size   int    `json:"size,omitempty"`
	Type   string `json:"type"`
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs,omitempty"`
	Outputs         []interface{} `json:"outputs,omitempty"`
	StateMutability string        `json:"stateMutability,omitempty"`
}

func NewGateway(chainId ...string) (sg StarknetGateway) {
	sg = StarknetGateway{"https://alpha4.starknet.io", "https://alpha4.starknet.io/feeder_gateway", "https://alpha4.starknet.io/gateway", UTF8StrToBig("SN_GOERLI")}
	if len(chainId) == 1 {
		if chainId[0] == "mainnet" || chainId[0] == "main" || chainId[0] == "SN_MAIN" {
			sg = StarknetGateway{"https://alpha-mainnet.starknet.io", "https://alpha-mainnet.starknet.io/feeder_gateway", "https://alpha-mainnet.starknet.io/gateway", UTF8StrToBig("SN_MAIN")}
		}
	}
	return sg
}

func (sg StarknetGateway) GetBlockHashById(blockId string) (block string, err error) {
	url := fmt.Sprintf("%s/get_block_hash_by_id?blockId=%s", sg.Feeder, blockId)

	resp, err := getHelper(url)
	if err != nil {
		return block, err
	}

	return strings.Replace(string(resp), "\"", "", -1), nil
}

func (sg StarknetGateway) GetBlockIdByHash(blockHash string) (block string, err error) {
	url := fmt.Sprintf("%s/get_block_id_by_hash?blockHash=%s", sg.Feeder, blockHash)

	resp, err := getHelper(url)
	if err != nil {
		return block, err
	}

	return strings.Replace(string(resp), "\"", "", -1), nil
}

func (sg StarknetGateway) GetTransactionHashById(txId string) (tx string, err error) {
	url := fmt.Sprintf("%s/get_transaction_hash_by_id?transactionId=%s", sg.Feeder, txId)

	resp, err := getHelper(url)
	if err != nil {
		return tx, err
	}

	return strings.Replace(string(resp), "\"", "", -1), nil
}

func (sg StarknetGateway) GetTransactionIdByHash(txHash string) (tx string, err error) {
	url := fmt.Sprintf("%s/get_transaction_id_by_hash?transactionHash=%s", sg.Feeder, txHash)

	resp, err := getHelper(url)
	if err != nil {
		return tx, err
	}

	return strings.Replace(string(resp), "\"", "", -1), nil
}

func (sg StarknetGateway) GetStorageAt(contractAddress, key, blockId string) (storage string, err error) {
	url := fmt.Sprintf("%s/get_storage_at?contractAddress=%s&key=%s%s", sg.Feeder, contractAddress, key, fmtBlockId(blockId))

	resp, err := getHelper(url)
	if err != nil {
		return storage, err
	}

	return strings.Replace(string(resp), "\"", "", -1), nil
}

func (sg StarknetGateway) GetCode(contractAddress, blockId string) (code ContractCode, err error) {
	url := fmt.Sprintf("%s/get_code?contractAddress=%s%s", sg.Feeder, contractAddress, fmtBlockId(blockId))

	resp, err := getHelper(url)
	if err != nil {
		return code, err
	}

	err = json.Unmarshal(resp, &code)
	return code, err
}

func (sg StarknetGateway) GetBlock(blockId string) (block Block, err error) {
	bid := fmtBlockId(blockId)

	url := fmt.Sprintf("%s/get_block%s", sg.Feeder, strings.Replace(bid, "&", "?", 1))

	resp, err := getHelper(url)
	if err != nil {
		return block, err
	}

	err = json.Unmarshal(resp, &block)
	return block, err
}

func (sg StarknetGateway) GetTransactionStatus(txHash string) (status TransactionStatus, err error) {
	url := fmt.Sprintf("%s/get_transaction_status?transactionHash=%s", sg.Feeder, txHash)

	resp, err := getHelper(url)
	if err != nil {
		return status, err
	}

	err = json.Unmarshal(resp, &status)
	return status, err
}

func (sg StarknetGateway) GetTransaction(txHash string) (tx StarknetTransaction, err error) {
	url := fmt.Sprintf("%s/get_transaction?transactionHash=%s", sg.Feeder, txHash)

	resp, err := getHelper(url)
	if err != nil {
		return tx, err
	}

	err = json.Unmarshal(resp, &tx)
	return tx, err
}

func (sg StarknetGateway) GetTransactionReceipt(txHash string) (receipt TransactionReceipt, err error) {
	url := fmt.Sprintf("%s/get_transaction_receipt?transactionHash=%s", sg.Feeder, txHash)

	resp, err := getHelper(url)
	if err != nil {
		return receipt, err
	}

	err = json.Unmarshal(resp, &receipt)
	return receipt, err
}

func fmtBlockId(blockId string) string {
	if len(blockId) < 2 {
		return ""
	}

	if blockId[:2] == "0x" {
		return fmt.Sprintf("&blockHash=%s", blockId)
	}
	return fmt.Sprintf("&blockNumber=%s", blockId)
}

func getHelper(url string) (resp []byte, err error) {
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return resp, err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	defer res.Body.Close()

	resp, err = ioutil.ReadAll(res.Body)
	return resp, err
}
