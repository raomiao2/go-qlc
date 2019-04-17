package consensus

import (
	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/types"
)

const (
	blockChanSize = 1024
)

type PovBlockSource struct {
	block *types.PovBlock
	from  types.PovBlockFrom
}

type PovBlockProcessor struct {
	povEngine *PoVEngine

	orphanBlocks  map[types.Hash]*PovBlockSource
	parentOrphans map[types.Hash][]*PovBlockSource

	blockCh chan *PovBlockSource
	quitCh  chan struct{}
}

func NewPovBlockProcessor(povEngine *PoVEngine) *PovBlockProcessor {
	bp := &PovBlockProcessor{
		povEngine: povEngine,
	}

	bp.orphanBlocks = make(map[types.Hash]*PovBlockSource)
	bp.parentOrphans = make(map[types.Hash][]*PovBlockSource)

	bp.blockCh = make(chan *PovBlockSource, blockChanSize)
	bp.quitCh = make(chan struct{})

	return bp
}

func (bp *PovBlockProcessor) Start() error {
	common.Go(bp.loop)
	return nil
}

func (bp *PovBlockProcessor) Init() error {
	return nil
}

func (bp *PovBlockProcessor) Stop() error {
	close(bp.quitCh)
	return nil
}

func (bp *PovBlockProcessor) AddBlock(block *types.PovBlock, from types.PovBlockFrom) error {
	bp.blockCh <- &PovBlockSource{block: block, from: from}
	return nil
}

func (bp *PovBlockProcessor) loop() {
	for {
		select {
		case block := <-bp.blockCh:
			bp.processBlock(block)
		case <-bp.quitCh:
			bp.povEngine.GetLogger().Info("Exiting process blocks")
			return
		}
	}
}

func (bp *PovBlockProcessor) processBlock(blockSrc *PovBlockSource) error {
	block := blockSrc.block
	blockHash := blockSrc.block.GetHash()
	bp.povEngine.GetLogger().Infof("process block, hash %s, height %d", blockHash, blockSrc.block.GetHeight())

	chain := bp.povEngine.GetChain()

	// duplicate block
	if chain.HasBlock(blockHash, block.GetHeight()) {
		return nil
	}

	// orphan block
	prevBlock := chain.GetBlockByHash(block.GetPrevious())
	if prevBlock == nil {
		bp.orphanBlocks[blockHash] = blockSrc
		bp.parentOrphans[block.GetPrevious()] = append(bp.parentOrphans[block.GetPrevious()], blockSrc)
		return nil
	}

	return nil
}
