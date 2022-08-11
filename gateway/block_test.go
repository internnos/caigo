package gateway

import (
	"context"
	"math/big"
	"testing"

	"github.com/dontpanicdao/caigo/types"
	"github.com/google/go-querystring/query"
)

func TestValueWithFelt(t *testing.T) {
	v := &BlockOptions{
		BlockNumber: 0,
		BlockHash:   &types.Felt{Int: big.NewInt(1)},
	}
	output, err := query.Values(v)
	if err != nil {
		t.Error(err)
	}
	x := output.Get("blockhash")
	if x != "1" {
		t.Errorf("Blockhash should be 1 (or 0x1), instead: \"%s\"", x)
	}
}

func Test_BlockIDByHash(t *testing.T) {
	gw := NewClient()

	id, err := gw.BlockIDByHash(context.Background(), "0x5239614da0a08b53fa8cbdbdcb2d852e153027ae26a2ae3d43f7ceceb28551e")
	if err != nil || id == 0 {
		t.Errorf("Getting Block ID by Hash: %v", err)
	}

	if id != 159179 {
		t.Errorf("Wrong Block ID from Hash: %v", err)
	}
}

func Test_BlockHashByID(t *testing.T) {
	gw := NewClient()

	id, err := gw.BlockHashByID(context.Background(), 159179)
	if err != nil || id == "" {
		t.Errorf("Getting Block ID by Hash: %v", err)
	}

	if id != "0x5239614da0a08b53fa8cbdbdcb2d852e153027ae26a2ae3d43f7ceceb28551e" {
		t.Errorf("Wrong Block ID from Hash: %v", err)
	}
}
