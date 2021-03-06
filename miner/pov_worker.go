package miner

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/event"
	"github.com/qlcchain/go-qlc/common/merkle"
	"github.com/qlcchain/go-qlc/common/statedb"
	"github.com/qlcchain/go-qlc/common/topic"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/vmcontract/contractaddress"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/log"
)

type PovWorker struct {
	miner  *Miner
	logger *zap.SugaredLogger

	maxTxPerBlock int

	minerAddr     types.Address
	minerAccount  *types.Account
	algoType      types.PovAlgoType
	cpuMining     bool
	mineWorkerNum int

	mineBlockPool   map[types.Hash]*types.PovMineBlock
	minerAlgoBlocks map[types.Address]map[types.PovAlgoType]*PovMinerAlgoBlock
	lastMineHeight  uint64
	muxMineBlock    sync.Mutex

	quitCh         chan struct{}
	feb            *event.FeedEventBus
	febRpcMsgCh    chan *topic.EventRPCSyncCallMsg
	febRpcMsgSubID event.FeedSubscription
}

type PovMinerAlgoBlock struct {
	curMineBlock *types.PovMineBlock
	lastMineTime time.Time
}

func NewPovWorker(cc *context.ChainContext, miner *Miner) *PovWorker {
	worker := &PovWorker{
		miner:  miner,
		logger: log.NewLogger("pov_miner"),
		quitCh: make(chan struct{}),
		feb:    cc.FeedEventBus(),

		febRpcMsgCh: make(chan *topic.EventRPCSyncCallMsg, 100),
	}
	worker.mineBlockPool = make(map[types.Hash]*types.PovMineBlock)
	worker.minerAlgoBlocks = make(map[types.Address]map[types.PovAlgoType]*PovMinerAlgoBlock)

	return worker
}

func (w *PovWorker) Init() error {
	blkHeader := &types.PovHeader{
		AuxHdr: types.NewPovAuxHeader(),
		CbTx:   types.NewPovCoinBaseTx(1, 2),
	}
	blkHdrSize := blkHeader.Msgsize()
	blkHdrSize += common.PovMaxCoinbaseExtraSize // cbtx extra

	hash := &types.Hash{}
	blkHdrSize += hash.Msgsize() * 32 * 2 // aux merkle branch + coinbase branch

	tx := &types.PovTransaction{}
	w.maxTxPerBlock = (common.PovChainBlockSize - blkHdrSize) / tx.Msgsize()
	w.maxTxPerBlock = w.maxTxPerBlock - 1 // CoinBase TX
	w.logger.Infof("MaxBlockSize:%d, MaxHeaderSize:%d, MaxTxSize:%d, MaxTxNum:%d",
		common.PovChainBlockSize, blkHdrSize, tx.Msgsize(), w.maxTxPerBlock)

	w.algoType = types.ALGO_UNKNOWN
	w.mineWorkerNum = 1

	cfg := w.GetConfig()
	if cfg.PoV.Coinbase != "" {
		var err error
		w.minerAddr, err = types.HexToAddress(cfg.PoV.Coinbase)
		if err != nil {
			w.logger.Errorf("invalid coinbase address %s", cfg.PoV.Coinbase)
			w.minerAddr = types.ZeroAddress
		}
	}
	if cfg.PoV.AlgoName != "" {
		w.algoType = types.NewPoVHashAlgoFromStr(cfg.PoV.AlgoName)
		if w.algoType == types.ALGO_UNKNOWN {
			w.logger.Errorf("invalid algo name %s", cfg.PoV.AlgoName)
			w.algoType = types.ALGO_SHA256D
		}
	}

	return nil
}

func (w *PovWorker) Start() error {
	cfg := w.GetConfig()

	w.febRpcMsgSubID = w.feb.Subscribe(topic.EventRpcSyncCall, w.febRpcMsgCh)

	common.Go(w.loop)

	if cfg.PoV.MinerEnabled && w.algoType != types.ALGO_UNKNOWN && !w.minerAddr.IsZero() {
		w.cpuMining = true
		common.Go(w.cpuMiningLoop)
	}

	return nil
}

func (w *PovWorker) Stop() error {
	w.febRpcMsgSubID.Unsubscribe()

	if w.quitCh != nil {
		close(w.quitCh)
	}

	return nil
}

func (w *PovWorker) GetConfig() *config.Config {
	return w.miner.GetConfig()
}

func (w *PovWorker) GetMinerAccount() *types.Account {
	if w.minerAccount != nil {
		return w.minerAccount
	}

	if w.minerAddr.IsZero() {
		return nil
	}

	accounts := w.miner.GetAccounts()
	for _, account := range accounts {
		if account.Address() == w.minerAddr {
			w.minerAccount = account
			return w.minerAccount
		}
	}

	return nil
}

func (w *PovWorker) GetMinerAddress() types.Address {
	return w.minerAddr
}

func (w *PovWorker) GetAlgoType() types.PovAlgoType {
	return w.algoType
}

func (w *PovWorker) loop() {
	for {
		select {
		case <-w.quitCh:
			return
		case msg := <-w.febRpcMsgCh:
			w.OnEventRpcSyncCall(msg)
		}
	}
}

func (w *PovWorker) OnEventRpcSyncCall(msg *topic.EventRPCSyncCallMsg) {
	needRsp := false
	switch msg.Name {
	case "Miner.GetWork":
		w.GetWork(msg.In, msg.Out)
		needRsp = true
	case "Miner.SubmitWork":
		w.SubmitWork(msg.In, msg.Out)
		needRsp = true
	case "Miner.GetMiningInfo":
		w.GetMiningInfo(msg.In, msg.Out)
		needRsp = true
	case "Miner.StartMining":
		w.StartMining(msg.In, msg.Out)
		needRsp = true
	case "Miner.StopMining":
		w.StopMining(msg.In, msg.Out)
		needRsp = true
	}
	if needRsp && msg.ResponseChan != nil {
		msg.ResponseChan <- msg.Out
	}
}

func (w *PovWorker) GetWork(in interface{}, out interface{}) {
	inArgs := in.(map[interface{}]interface{})
	outArgs := out.(map[interface{}]interface{})

	if w.miner.GetSyncState() != topic.SyncDone {
		outArgs["err"] = fmt.Errorf("miner pausing for sync state %s", w.miner.GetSyncState())
		return
	}

	minerAddr := inArgs["minerAddr"].(types.Address)
	algoName := inArgs["algoName"].(string)
	algoType := types.NewPoVHashAlgoFromStr(algoName)
	if !common.PovIsAlgoSupported(algoType) {
		outArgs["err"] = errors.New("unknown algorithm name")
		return
	}

	err := w.checkMinerPledge(minerAddr)
	if err != nil {
		outArgs["err"] = err
		return
	}

	mineBlock, err := w.generateBlock(minerAddr, algoType)
	if err != nil {
		outArgs["err"] = err
		return
	}

	outArgs["err"] = nil
	outArgs["mineBlock"] = mineBlock
}

func (w *PovWorker) SubmitWork(in interface{}, out interface{}) {
	inArgs := in.(map[interface{}]interface{})
	outArgs := out.(map[interface{}]interface{})

	result := inArgs["mineResult"].(*types.PovMineResult)

	mineBlock := w.findBlockInPool(result.WorkHash)
	if mineBlock == nil {
		outArgs["err"] = errors.New("failed to find block by WorkHash")
		return
	}

	err := w.checkAndFillBlockByResult(mineBlock, result)
	if err != nil {
		outArgs["err"] = err
		return
	}

	w.submitBlock(mineBlock)

	outArgs["err"] = nil
}

func (w *PovWorker) StartMining(in interface{}, out interface{}) {
	inArgs := in.(map[interface{}]interface{})
	outArgs := out.(map[interface{}]interface{})

	if w.cpuMining {
		outArgs["err"] = errors.New("cpu mining has been enabled already")
		return
	}

	minerAddr := inArgs["minerAddr"].(types.Address)
	algoName := inArgs["algoName"].(string)
	algoType := types.NewPoVHashAlgoFromStr(algoName)

	if minerAddr.IsZero() && w.minerAddr.IsZero() {
		outArgs["err"] = errors.New("invalid miner address in request or config")
		return
	}

	if algoType == types.ALGO_UNKNOWN && w.algoType == types.ALGO_UNKNOWN {
		outArgs["err"] = errors.New("invalid algo name in request or config")
		return
	}

	if !minerAddr.IsZero() {
		err := w.checkMinerPledge(minerAddr)
		if err != nil {
			outArgs["err"] = err
			return
		}
	}

	if !minerAddr.IsZero() {
		w.minerAddr = minerAddr
	}

	if algoType != types.ALGO_UNKNOWN {
		w.algoType = algoType
	}

	if w.minerAddr.IsZero() || algoType == types.ALGO_UNKNOWN {
		outArgs["err"] = errors.New("invalid miner address or algo name")
		return
	}

	w.cpuMining = true
	common.Go(w.cpuMiningLoop)

	outArgs["err"] = nil
}

func (w *PovWorker) StopMining(in interface{}, out interface{}) {
	//inArgs := in.(map[interface{}]interface{})
	outArgs := out.(map[interface{}]interface{})

	if !w.cpuMining {
		outArgs["err"] = errors.New("cpu mining has been disabled already")
		return
	}

	w.cpuMining = false

	outArgs["err"] = nil
}

func (w *PovWorker) GetMiningInfo(in interface{}, out interface{}) {
	//inArgs := in.(map[interface{}]interface{})
	outArgs := out.(map[interface{}]interface{})

	outArgs["syncState"] = int(w.miner.GetSyncState())

	latestBlock := w.miner.GetChain().LatestBlock()

	outArgs["latestBlock"] = latestBlock
	outArgs["pooledTx"] = w.miner.GetTxPool().GetPendingTxNum()

	outArgs["minerAddr"] = w.minerAddr
	outArgs["minerAlgo"] = w.algoType
	outArgs["cpuMining"] = w.cpuMining

	outArgs["err"] = nil
}

func (w *PovWorker) newBlockTemplate(minerAddr types.Address, algoType types.PovAlgoType) (*types.PovMineBlock, error) {
	latestHeader := w.miner.GetChain().LatestHeader()
	if latestHeader == nil {
		return nil, fmt.Errorf("failed to get latest header")
	}

	w.logger.Debugf("make block template after latest %d/%s", latestHeader.GetHeight(), latestHeader.GetHash())

	mineBlock := types.NewPovMineBlock()
	if mineBlock.Header == nil || mineBlock.Block == nil {
		return nil, fmt.Errorf("failed to new block")
	}
	if mineBlock.Header.CbTx == nil {
		return nil, fmt.Errorf("failed to new coinbase tx")
	}

	// fill base header
	header := mineBlock.Header
	header.BasHdr.Version = types.POV_VBS_TOPBITS | uint32(algoType)
	header.BasHdr.Previous = latestHeader.GetHash()
	header.BasHdr.Height = latestHeader.GetHeight() + 1
	header.BasHdr.Timestamp = uint32(time.Now().Unix())

	prevStateHash := latestHeader.GetStateHash()
	gsdb := statedb.NewPovGlobalStateDB(w.miner.l.DBStore(), latestHeader.GetStateHash())
	if gsdb == nil {
		return nil, fmt.Errorf("failed to get state db %s", prevStateHash)
	}

	// coinbase tx
	cbtx := header.CbTx

	// pack account block txs
	accBlocks := w.miner.GetTxPool().SelectPendingTxs(gsdb, w.maxTxPerBlock)

	w.logger.Debugf("current block select pending txs %d", len(accBlocks))

	var accTxHashes []*types.Hash
	var accTxs []*types.PovTransaction
	for _, accBlock := range accBlocks {
		accTx := &types.PovTransaction{
			Hash:  accBlock.GetHash(),
			Block: accBlock,
		}
		accTxHashes = append(accTxHashes, &accTx.Hash)
		accTxs = append(accTxs, accTx)
	}

	err := w.miner.GetPovConsensus().PrepareHeader(header)
	if err != nil {
		return nil, err
	}

	err = w.miner.GetChain().TransitStateDB(header.GetHeight(), accTxs, gsdb)
	if err != nil {
		return nil, err
	}
	if gsdb.GetCurTrie() == nil {
		return nil, fmt.Errorf("failed to generate state trie")
	}

	// build coinbase tx
	cbtx = mineBlock.Header.CbTx
	cbtx.TxNum = uint32(len(accTxs) + 1)
	cbtx.StateHash = gsdb.GetCurHash()

	minerRwd, repRwd, err := w.miner.GetChain().CalcBlockReward(header)
	if err != nil {
		return nil, err
	}

	minerTxOut := cbtx.GetMinerTxOut()
	minerTxOut.Address = minerAddr
	minerTxOut.Value = minerRwd

	repTxOut := cbtx.GetRepTxOut()
	repTxOut.Address = contractaddress.MinerAddress
	repTxOut.Value = repRwd

	cbTxHash := cbtx.ComputeHash()

	// append all txs to body
	body := mineBlock.Body
	cbTxPov := &types.PovTransaction{Hash: cbTxHash, CbTx: cbtx}
	body.Txs = append(body.Txs, cbTxPov)
	body.Txs = append(body.Txs, accTxs...)

	// calc merkle root
	var mklTxHashList []*types.Hash
	mklTxHashList = append(mklTxHashList, &cbTxPov.Hash)
	mklTxHashList = append(mklTxHashList, accTxHashes...)
	mklHash := merkle.CalcMerkleTreeRootHash(mklTxHashList)
	header.BasHdr.MerkleRoot = mklHash

	mineBlock.AllTxHashes = mklTxHashList

	// calc merkle branch without coinbase tx
	mineBlock.CoinbaseBranch = merkle.BuildCoinbaseMerkleBranch(accTxHashes)

	mineBlock.WorkHash = mineBlock.Block.ComputeHash()
	mineBlock.MinTime = w.miner.GetChain().CalcPastMedianTime(latestHeader)
	return mineBlock, nil
}

func (w *PovWorker) generateBlock(minerAddr types.Address, algoType types.PovAlgoType) (*types.PovMineBlock, error) {
	w.muxMineBlock.Lock()
	defer w.muxMineBlock.Unlock()

	var err error

	latestHeader := w.miner.GetChain().LatestHeader()

	// reset all pending blocks when best chain changed
	if w.lastMineHeight != latestHeader.GetHeight() {
		w.mineBlockPool = make(map[types.Hash]*types.PovMineBlock)
		w.minerAlgoBlocks = make(map[types.Address]map[types.PovAlgoType]*PovMinerAlgoBlock)

		w.lastMineHeight = latestHeader.GetHeight()
	}

	var curMinerAlgoBlk *PovMinerAlgoBlock
	curMinerAlgos := w.minerAlgoBlocks[minerAddr]
	if curMinerAlgos == nil {
		curMinerAlgos = make(map[types.PovAlgoType]*PovMinerAlgoBlock)
		w.minerAlgoBlocks[minerAddr] = curMinerAlgos
	}
	curMinerAlgoBlk = curMinerAlgos[algoType]
	if curMinerAlgoBlk == nil {
		curMinerAlgoBlk = new(PovMinerAlgoBlock)
		curMinerAlgos[algoType] = curMinerAlgoBlk
	}

	if curMinerAlgoBlk.curMineBlock == nil ||
		time.Now().After(curMinerAlgoBlk.lastMineTime.Add(30*time.Second)) {
		curMinerAlgoBlk.curMineBlock, err = w.newBlockTemplate(minerAddr, algoType)
		if err != nil {
			return nil, err
		}
		curMinerAlgoBlk.lastMineTime = time.Now()

		w.mineBlockPool[curMinerAlgoBlk.curMineBlock.WorkHash] = curMinerAlgoBlk.curMineBlock
	}

	return curMinerAlgoBlk.curMineBlock, nil
}

func (w *PovWorker) findBlockInPool(workHash types.Hash) *types.PovMineBlock {
	w.muxMineBlock.Lock()
	defer w.muxMineBlock.Unlock()

	return w.mineBlockPool[workHash]
}

func (w *PovWorker) checkAndFillBlockByResult(mineBlock *types.PovMineBlock, result *types.PovMineResult) error {
	if len(result.CoinbaseExtra) < common.PovMinCoinbaseExtraSize {
		return fmt.Errorf("coinbase extra size too small, min size is %d", common.PovMinCoinbaseExtraSize)
	}

	if len(result.CoinbaseExtra) > common.PovMaxCoinbaseExtraSize {
		return fmt.Errorf("coinbase extra size too big, max size is %d", common.PovMaxCoinbaseExtraSize)
	}

	mineBlock.Header.CbTx.TxIns[0].Extra = result.CoinbaseExtra
	cbTxHash := mineBlock.Header.CbTx.ComputeHash()
	if cbTxHash.Cmp(result.CoinbaseHash) != 0 {
		return fmt.Errorf("coinbase hash not equal, %s != %s", cbTxHash, result.CoinbaseHash)
	}

	mineBlock.Header.CbTx.Hash = result.CoinbaseHash

	cbTxPov := mineBlock.Body.Txs[0]
	cbTxPov.Hash = result.CoinbaseHash
	mineBlock.AllTxHashes[0] = &cbTxPov.Hash

	calcMklRoot := merkle.CalcMerkleTreeRootHash(mineBlock.AllTxHashes)
	if calcMklRoot.Cmp(result.MerkleRoot) != 0 {
		return fmt.Errorf("merkle root not equal, %s != %s", calcMklRoot, result.MerkleRoot)
	}
	mineBlock.Header.BasHdr.MerkleRoot = result.MerkleRoot

	mineBlock.Header.BasHdr.Timestamp = result.Timestamp
	mineBlock.Header.BasHdr.Nonce = result.Nonce

	calcBlkHash := mineBlock.Header.ComputeHash()
	if calcBlkHash.Cmp(result.BlockHash) != 0 {
		return fmt.Errorf("block hash not equal, %s != %s", calcBlkHash, result.BlockHash)
	}
	mineBlock.Header.BasHdr.Hash = result.BlockHash

	if result.AuxPow != nil {
		calcParHash := result.AuxPow.ParBlockHeader.ComputeHash()
		if calcParHash != result.AuxPow.ParentHash {
			return fmt.Errorf("parent block hash not equal, %s != %s", calcParHash, result.AuxPow.ParentHash)
		}
		mineBlock.Header.AuxHdr = result.AuxPow
	}

	return nil
}

func (w *PovWorker) checkMinerPledge(minerAddr types.Address) error {
	cfg := w.miner.GetConfig()
	if cfg.PoV.ChainParams.MinerPledge.Sign() <= 0 {
		return nil
	}
	pledgeAmount := cfg.PoV.ChainParams.MinerPledge

	latestBlock := w.miner.GetChain().LatestBlock()

	if latestBlock.GetHeight() >= (common.PovMinerVerifyHeightStart - 1) {
		prevStateHash := latestBlock.GetStateHash()
		gsdb := statedb.NewPovGlobalStateDB(w.miner.l.DBStore(), prevStateHash)
		if gsdb == nil {
			return errors.New("miner pausing for get state db failed")
		}
		rs, _ := gsdb.GetRepState(minerAddr)
		if rs == nil {
			return errors.New("miner pausing for account state not exist")
		}
		if rs.Vote.Compare(pledgeAmount) == types.BalanceCompSmaller {
			return errors.New("miner pausing for vote pledge not enough")
		}
	}

	return nil
}

func (w *PovWorker) submitBlock(mineBlock *types.PovMineBlock) {
	newBlock := mineBlock.Block.Copy()

	w.logger.Infof("submit block %d/%s", newBlock.GetHeight(), newBlock.GetHash())

	var retErr error

	msg := &topic.EventPovRecvBlockMsg{
		Block:        newBlock,
		From:         types.PovBlockFromLocal,
		MsgPeer:      "",
		ResponseChan: make(chan interface{}, 1),
	}
	w.miner.eb.Publish(topic.EventPovRecvBlock, msg)

	select {
	case retVal := <-msg.ResponseChan:
		if retVal != nil {
			if err, ok := retVal.(error); ok {
				retErr = err
			}
		}
	case <-time.After(time.Minute):
		retErr = errors.New("ResponseChan timeout")
	}

	if retErr != nil {
		w.logger.Warnf("failed to submit block %d/%s", newBlock.GetHeight(), newBlock.GetHash())
	}
}
