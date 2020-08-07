package main
/*
import (
    "context"
    "encoding/hex"
    "fmt"
    "log"

    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/rlp"
)

func main() {
    client, err := ethclient.Dial("https://ropsten.infura.io/v3/375a84d45ba0456a8d39a32cce31471c")
    if err != nil {
        log.Fatal(err)
    }

    rawTx := "f86b808502540be40082520894743376fd2a693723a60942d0b4b2f1765ea1dbb087038d7ea4c680008029a0149d5ada14f2afbafc81e2b166f8eebd078bdd49c93fb240fbfc1e156e96cf52a00c8910089e4a97c33b3f35c35d86c7d0b2df9b736516740ea095d3d51282"

    var tx *types.Transaction

	rawTxBytes, err := hex.DecodeString(rawTx)
	rlp.DecodeBytes(rawTxBytes, &tx)

    err = client.SendTransaction(context.Background(), tx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("tx sent: %s", tx.Hash().Hex()) // tx sent: 0xc429e5f128387d224ba8bed6885e86525e14bfdc2eb24b5e9c3351a1176fd81f
}
*/