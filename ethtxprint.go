package ethtxprint

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/juztin/ethunit"
)

type status uint8

const (
	StatusUnknown = iota
	StatusPending
	StatusFailed
	StatusSuccessful
)

func (s status) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusFailed:
		return "Failed"
	case StatusSuccessful:
		return "Succesful"
	default:
		return "Unknown"
	}
}

func txTypeMessage(t uint8) string {
	switch t {
	case 0x0:
		return "Legacy"
	case 0x2:
		return "EIP-1559"
	default:
		return "Unknown"
	}
}

type Transaction struct {
	hash           common.Hash
	status         status
	block          *big.Int
	blockIndex     uint
	blockTime      time.Time
	confirmations  uint64
	from           *common.Address
	to             *common.Address
	value          *big.Int
	txFee          *big.Int
	gasPrice       *big.Int
	txType         uint8
	gasLimit       uint64
	gasUsed        *big.Int
	gasUsedPct     float64
	baseFee        *big.Int
	maxFee         *big.Int
	maxPriorityFee *big.Int
	burntFees      *big.Int
	txSavings      *big.Int
	nonce          uint64
	data           []byte
}

func (t *Transaction) String() string {
	var (
		blockMsg, blockDur, blockTime string
		timeMsg                       string
		txFeeMsg                      string
		gasUsedMsg                    string
		baseFeeMsg                    string
		maxFeeMsg, maxPriorityMsg     string
		burnSavingsMsg                string
		noncePosition                 string
	)
	if t.status == StatusPending {
		blockMsg = "(Pending)"
		blockDur = "Pending"
		timeMsg = "Pending"
		txFeeMsg = "0"
		gasUsedMsg = "Pending"
		baseFeeMsg = "0"
		noncePosition = "Pending"

	} else {
		blockTime = t.blockTime.String()
		d := time.Now().UTC().Sub(t.blockTime)
		switch hours := d.Hours(); {
		case hours > 24:
			t := time.Time{}.Add(d)
			blockDur = fmt.Sprintf("%d days %d hours ago", t.Day()-1, t.Hour())
		case hours > 0:
			t := time.Time{}.Add(d)
			blockDur = fmt.Sprintf("%d hours %d minutes ago", t.Hour(), t.Minute())
		case d.Minutes() > 0:
			blockDur = fmt.Sprintf("%d mins ago", d.Minutes())
		default:
			blockDur = fmt.Sprintf("%d seconds ago", d.Seconds())
		}
		timeMsg = fmt.Sprintf("%s (%s)", blockDur, blockTime)
		blockMsg = fmt.Sprintf("%s (%d confirmations)", t.block.String(), t.confirmations)
		txFeeMsg = ethunit.WeiToEther(t.txFee).Text('f', -1)
		gasUsedMsg = fmt.Sprintf("%d (%.2f%%)", t.gasUsed, t.gasUsedPct)
		baseFeeMsg = fmt.Sprintf("%d Wei (%s Gwei)", t.baseFee.Int64(), ethunit.WeiToGwei(t.baseFee).Text('f', -1))
		noncePosition = strconv.FormatUint(uint64(t.blockIndex), 10)
		var savings string
		if t.txType == 0x2 {
			savings = fmt.Sprintf("\nTxn Savings:              %s Ether", ethunit.WeiToEther(t.txSavings).Text('f', -1))
		}
		burnSavingsMsg = fmt.Sprintf("\nBurnt Fees:               %s Ether%s", ethunit.WeiToEther(t.burntFees).Text('f', -1), savings)
	}

	if t.txType == 0x2 {
		maxFeeMsg = fmt.Sprintf("\nMax Fee Per Gas:          %s Ether (%s Gwei)", ethunit.WeiToEther(t.maxFee).Text('f', -1), ethunit.WeiToGwei(t.maxFee).Text('f', -1))
		maxPriorityMsg = fmt.Sprintf("\nMax Priority Fee Per Gas: %s Ether (%s Gwei)", ethunit.WeiToEther(t.maxPriorityFee).Text('f', -1), ethunit.WeiToGwei(t.maxPriorityFee).Text('f', -1))
	}

	return fmt.Sprintf(`Transaction Hash:         %s
Block:                    %s
Timestamp:                %s
From:                     %s
To:                       %s
Value:                    %s Ether
Transaction Fee:          %s Ether
Gas Price:                %s Ether (%s Gwei)
Txn Type:                 %d (%s)
Gas Limit:                %d
Gas Used By Transaction:  %s
Base Fee Per Gas:         %s%s%s%s
Nonce (position):         %d (%s)
Input Data:               %x
`,
		t.hash.String(),
		blockMsg,
		timeMsg,
		t.from.String(),
		t.to.String(),
		ethunit.WeiToGwei(t.value).Text('f', -1),
		txFeeMsg,
		ethunit.WeiToEther(t.gasPrice).Text('f', -1), ethunit.WeiToGwei(t.gasPrice).Text('f', -1),
		t.txType, txTypeMessage(t.txType),
		t.gasLimit,
		gasUsedMsg,
		baseFeeMsg, maxFeeMsg, maxPriorityMsg, burnSavingsMsg,
		t.nonce, noncePosition,
		t.data)
}

func NewTransaction(ctx context.Context, c *ethclient.Client, hash common.Hash) (*Transaction, error) {
	t := &Transaction{hash: hash}
	tx, pending, err := c.TransactionByHash(ctx, t.hash)
	if err != nil {
		return t, err
	}
	head, err := c.BlockByNumber(ctx, nil)
	if err != nil {
		return t, err
	}

	t.to = tx.To()
	t.value = tx.Value()
	t.txType = tx.Type()
	t.gasLimit = tx.Gas()
	t.nonce = tx.Nonce()
	t.data = tx.Data()

	var m types.Message
	switch t.txType {
	case 0x0:
		m, err = tx.AsMessage(types.NewEIP155Signer(tx.ChainId()), t.maxFee)
	case 0x2:
		m, err = tx.AsMessage(types.NewLondonSigner(tx.ChainId()), t.maxFee)
	}
	if err != nil {
		return t, nil
	}
	from := m.From()
	t.from = &from

	if pending {
		t.status = StatusPending
		t.gasPrice = big.NewInt(0).Add(tx.GasTipCap(), head.BaseFee())
		t.maxFee = big.NewInt(0)
		t.maxPriorityFee = big.NewInt(0)
		return t, nil
	}

	r, err := c.TransactionReceipt(ctx, hash)
	if err != nil {
		return t, err
	}
	b, err := c.BlockByHash(ctx, r.BlockHash)
	if err != nil {
		return t, err
	}

	switch r.Status {
	case 0x0:
		t.status = StatusFailed
	case 0x1:
		t.status = StatusSuccessful
	default:
		t.status = StatusUnknown
	}

	t.confirmations = head.NumberU64() - r.BlockNumber.Uint64()
	t.block = b.Number()
	t.blockIndex = r.TransactionIndex
	t.blockTime = time.Unix(int64(b.Time()), 0)
	t.gasPrice = big.NewInt(0).Add(tx.GasTipCap(), b.BaseFee())
	t.txFee = big.NewInt(0).Mul(t.gasPrice, new(big.Int).SetUint64(r.GasUsed))
	t.baseFee = b.BaseFee()
	t.gasUsed = new(big.Int).SetUint64(r.GasUsed)
	t.gasUsedPct = float64(r.GasUsed) / float64(t.gasLimit) * 100
	t.burntFees = big.NewInt(0).Mul(t.gasUsed, t.baseFee)
	t.maxFee = tx.GasFeeCap()
	t.maxPriorityFee = tx.GasTipCap()
	if t.txType == 0x2 {
		// (Max Fee Per Gas - (Base Fee Per Gas + Max Priority Fee Per Gas)) * Gas Used
		s := big.NewInt(0)
		s.Mul(t.gasUsed, s.Sub(t.maxFee, s.Add(t.baseFee, t.maxPriorityFee)))
		t.txSavings = s
	}

	return t, nil
}
