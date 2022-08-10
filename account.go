package caigo

import (
	"context"
	"math/big"

	"github.com/dontpanicdao/caigo/types"
)

const (
	EXECUTE_SELECTOR    string = "__execute__"
	TRANSACTION_PREFIX  string = "invoke"
	TRANSACTION_VERSION int64  = 0
)

type Account struct {
	Provider types.Provider
	Address  types.Felt
	PublicX  *big.Int
	PublicY  *big.Int
	private  *big.Int
}

/*
Instantiate a new StarkNet Account which includes structures for calling the network and signing transactions:
- private signing key
- stark curve definition
- full provider definition
- public key pair for signature verifications
*/
func NewAccount(private string, address *types.Felt, provider types.Provider) (*Account, error) {
	priv := SNValToBN(private)
	x, y, err := Curve.PrivateToPoint(priv)
	if err != nil {
		return nil, err
	}

	return &Account{
		Provider: provider,
		Address:  *address,
		PublicX:  x,
		PublicY:  y,
		private:  priv,
	}, nil
}

func (account *Account) Sign(msgHash *big.Int) (*big.Int, *big.Int, error) {
	return Curve.Sign(msgHash, account.private)
}

/*
invocation wrapper for StarkNet account calls to '__execute__' contact calls through an account abstraction
- implementation has been tested against OpenZeppelin Account contract as of: https://github.com/OpenZeppelin/cairo-contracts/blob/4116c1ecbed9f821a2aa714c993a35c1682c946e/src/openzeppelin/account/Account.cairo
- accepts a multicall
*/
func (account *Account) Execute(ctx context.Context, maxFee *types.Felt, calls []types.Transaction) (*types.AddTxResponse, error) {
	req, err := account.fmtExecute(ctx, maxFee, calls)
	if err != nil {
		return nil, err
	}

	return account.Provider.Invoke(ctx, *req)
}

func (account *Account) HashMultiCall(fee *types.Felt, nonce *types.Felt, calls []types.Transaction) (*big.Int, error) {
	chainID, err := account.Provider.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	callArray := fmtExecuteCalldata(nonce, calls)
	callArray = append(callArray, types.BigToFelt(big.NewInt(int64(len(callArray)))))
	cdHash, err := Curve.HashElements(callArray)
	if err != nil {
		return nil, err
	}

	multiHashData := []*big.Int{
		UTF8StrToBig(TRANSACTION_PREFIX),
		big.NewInt(TRANSACTION_VERSION),
		SNValToBN(account.Address.String()),
		GetSelectorFromName(EXECUTE_SELECTOR),
		cdHash,
		fee.Int,
		UTF8StrToBig(chainID),
	}

	multiHashData = append(multiHashData, big.NewInt(int64(len(multiHashData))))
	return Curve.HashElements(multiHashData)
}

func (account *Account) EstimateFee(ctx context.Context, calls []types.Transaction) (*types.FeeEstimate, error) {
	zeroFee := &types.Felt{Int: big.NewInt(0)}

	req, err := account.fmtExecute(ctx, zeroFee, calls)
	if err != nil {
		return nil, err
	}

	return account.Provider.EstimateFee(ctx, *req, "")
}

func (account *Account) fmtExecute(ctx context.Context, maxFee *types.Felt, calls []types.Transaction) (*types.FunctionInvoke, error) {
	nonce, err := account.Provider.AccountNonce(ctx, account.Address.String())
	if err != nil {
		return nil, err
	}

	req := types.FunctionInvoke{
		FunctionCall: types.FunctionCall{
			ContractAddress:    &account.Address,
			EntryPointSelector: types.StrToFelt(EXECUTE_SELECTOR),
			Calldata:           fmtExecuteCalldataStrings(nonce, calls),
		},
		MaxFee: maxFee,
	}

	hash, err := account.HashMultiCall(maxFee, nonce, calls)
	if err != nil {
		return nil, err
	}

	r, s, err := account.Sign(hash)
	if err != nil {
		return nil, err
	}
	req.Signature = types.Signature{types.BigToFelt(r), types.BigToFelt(s)}

	return &req, nil
}

func fmtExecuteCalldataStrings(nonce *types.Felt, calls []types.Transaction) (calldataStrings []*types.Felt) {
	callArray := fmtExecuteCalldata(nonce, calls)
	for _, data := range callArray {
		calldataStrings = append(calldataStrings, data)
	}
	return calldataStrings
}

/*
Formats the multicall transactions in a format which can be signed and verified by the network and OpenZeppelin account contracts
*/
func fmtExecuteCalldata(nonce *types.Felt, calls []types.Transaction) (calldataArray []*types.Felt) {
	callArray := make([]*types.Felt, len(calls))

	for _, tx := range calls {
		callArray = append(callArray, tx.ContractAddress, tx.EntryPointSelector)

		if len(tx.Calldata) == 0 {
			callArray = append(callArray, types.StrToFelt("0"), types.StrToFelt("0"))
			continue
		}

		callArray = append(callArray, types.BigToFelt(big.NewInt(int64(len(calldataArray)))), types.BigToFelt(big.NewInt(int64(len(tx.Calldata)))))
		for _, cd := range tx.Calldata {
			calldataArray = append(calldataArray, cd)
		}
	}

	callArray = append(callArray, types.BigToFelt(big.NewInt(int64(len(calldataArray)))))
	callArray = append(callArray, calldataArray...)
	callArray = append(callArray, nonce)
	return callArray
}
