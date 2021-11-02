package core

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

type BlockReplicationEvent struct {
	Hash string
	Data []byte
}

func (bc *BlockChain) createBlockReplica(block *types.Block, config *params.ChainConfig, stateSpecimen *types.StateSpecimen) error {
	//block replica
	exportBlockReplica, err := bc.createReplica(block, config, stateSpecimen)
	if err != nil {
		return err
	}
	//encode to rlp
	blockReplicaRLP, err := rlp.EncodeToBytes(exportBlockReplica)
	if err != nil {
		return err
	}

	sHash := block.Hash().String()

	log.Info("Creating block replication event", "block number", block.NumberU64(), "hash", sHash)
	bc.blockReplicationFeed.Send(BlockReplicationEvent{
		sHash,
		blockReplicaRLP,
	})

	return nil
}

func (bc *BlockChain) createReplica(block *types.Block, config *params.ChainConfig, stateSpecimen *types.StateSpecimen) (*types.ExportBlockReplica, error) {

	bHash := block.Hash()
	bNum := block.NumberU64()

	//totalDifficulty
	tdRLP := rawdb.ReadTdRLP(bc.db, bHash, bNum)
	td := new(big.Int)
	if err := rlp.Decode(bytes.NewReader(tdRLP), td); err != nil {
		log.Error("Invalid block total difficulty RLP ", "hash ", bHash, "err", err)
		return nil, err
	}

	//header
	headerRLP := rawdb.ReadHeaderRLP(bc.db, bHash, bNum)
	header := new(types.Header)
	if err := rlp.Decode(bytes.NewReader(headerRLP), header); err != nil {
		log.Error("Invalid block header RLP ", "hash ", bHash, "err ", err)
		return nil, err
	}

	//transactions
	txsExp := make([]*types.TransactionForExport, len(block.Transactions()))
	txsRlp := make([]*types.TransactionExportRLP, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		txsExp[i] = (*types.TransactionForExport)(tx)
		txsRlp[i] = txsExp[i].ExportTx()
	}

	//receipts
	receipts := rawdb.ReadRawReceipts(bc.db, bHash, bNum)
	receiptsExp := make([]*types.ReceiptForExport, len(receipts))
	receiptsRlp := make([]*types.ReceiptExportRLP, len(receipts))
	for i, receipt := range receipts {
		receiptsExp[i] = (*types.ReceiptForExport)(receipt)
		receiptsRlp[i] = receiptsExp[i].ExportReceipt()
	}

	//senders
	signer := types.MakeSigner(bc.chainConfig, block.Number())
	senders := make([]common.Address, 0, len(block.Transactions()))
	for _, tx := range block.Transactions() {
		sender, err := types.Sender(signer, tx)
		if err != nil {
			return nil, err
		} else {
			senders = append(senders, sender)
		}
	}

	//uncles
	uncles := block.Uncles()

	//block specimen export
	exportBlockReplica := &types.ExportBlockReplica{
		Type:         "block-replica",
		NetworkId:    config.ChainID.Uint64(),
		Hash:         bHash,
		TotalDiff:    td,
		Header:       header,
		Transactions: txsRlp,
		Uncles:       uncles,
		Receipts:     receiptsRlp,
		Senders:      senders,
		State:        stateSpecimen,
	}
	return exportBlockReplica, nil
}

// SubscribeChainReplicationEvent registers a subscription of ChainReplicationEvent.
func (bc *BlockChain) SubscribeBlockReplicationEvent(ch chan<- BlockReplicationEvent) event.Subscription {
	return bc.scope.Track(bc.blockReplicationFeed.Subscribe(ch))
}
