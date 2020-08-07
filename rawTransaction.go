package main

import (
    "context"
    "crypto/ecdsa"
    "encoding/hex"
    "fmt"
    "log"
    "math/big"

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/rlp"
	//"github.com/ethereum/go-ethereum/common/hexutil"
)

func main() {
    client, err := ethclient.Dial("https://ropsten.infura.io/v3/375a84d45ba0456a8d39a32cce31471c")
    if err != nil {
        log.Fatal(err)
    }
    privateKey, err := crypto.HexToECDSA("4BBE464C115B639F9AD2D858D4A84CB6D2185B8CB6F08BFC63809EFC863684CA")
    if err != nil {
        log.Fatal(err)
    }

    publicKey := privateKey.Public()
    publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
    if !ok {
        log.Fatal("error casting public key to ECDSA")
    }

    fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
    fmt.Printf("fromAddress: %d\n", fromAddress)
    nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
    if err != nil {
        log.Fatal(err)
    }
    //fmt.Printf("%d\n", nonce)
    //var nonce hexutil.Uint644
    //return uint64(nonce)
    //r: 113435503106888623780845422972399380807025119313940195470264666760611920129301 s: 47105817020856271236461922630442430066243070903082139113143856007077248751442

    value := big.NewInt(1000000000000000) // in wei (0.001 eth)
    gasLimit := uint64(21000)                // in units
    gasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%d\n", gasPrice)

    toAddress := common.HexToAddress("0x743376fd2a693723A60942D0b4B2F1765ea1Dbb0")
    var data []byte
    tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
    
    chainID, err := client.NetworkID(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
// s = types.NewEIP155Signer(chainID)
    signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
    if err != nil {
        log.Fatal(err)
    }
    r := new(big.Int)
    s := new(big.Int)
    v := new(big.Int)
    v, r, s = signedTx.RawSignatureValues()
    fmt.Printf("r: %d s: %d v: %d\n", r,s,v)

    ts := types.Transactions{signedTx}
    rawTx := hex.EncodeToString(ts.GetRlp(0))

    fmt.Printf(rawTx) // f86...772
    fmt.Printf("\n")
    //rawTx := "f86b808502540be40082520894743376fd2a693723a60942d0b4b2f1765ea1dbb087038d7ea4c680008029a0149d5ada14f2afbafc81e2b166f8eebd078bdd49c93fb240fbfc1e156e96cf52a00c8910089e4a97c33b3f35c35d86c7d0b2df9b736516740ea095d3d51282"
    var txs *types.Transaction

	rawTxBytes, err := hex.DecodeString(rawTx)
	rlp.DecodeBytes(rawTxBytes, &txs)

    err = client.SendTransaction(context.Background(), txs)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("tx sent: %s", txs.Hash().Hex()) // tx sent: 0xc429e5f128387d224ba8bed6885e86525e14bfdc2eb24b5e9c3351a1176fd81f

}
