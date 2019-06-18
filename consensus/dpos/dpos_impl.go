package dpos

import (
	"github.com/bluele/gcache"
	"github.com/qlcchain/go-qlc/consensus"
	"github.com/qlcchain/go-qlc/ledger/process"
	"sync"
	"time"

	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/event"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/log"
	"github.com/qlcchain/go-qlc/p2p"
	"github.com/qlcchain/go-qlc/p2p/protos"
	"go.uber.org/zap"
)

const (
	targetTps             = 100
	repTimeout            = 5 * time.Minute
	uncheckedCacheSize    = targetTps * 5 * 60
	uncheckedTimeout      = 5 * time.Minute
	voteCacheSize         = targetTps * 5 * 60
	voteCacheTimeout      = 5 * time.Minute
	refreshPriInterval    = 1 * time.Minute
	findOnlineRepInterval = 2 * time.Minute
	maxBlocks             = 1024000
)

var (
	localRepAccount sync.Map
)

type DPoS struct {
	ledger         *ledger.Ledger
	acTrx          *ActiveTrx
	accounts       []*types.Account
	onlineReps     sync.Map
	logger         *zap.SugaredLogger
	cfg            *config.Config
	eb             event.EventBus
	lv             *process.LedgerVerifier
	uncheckedCache gcache.Cache //gap blocks
	voteCache      gcache.Cache //vote blocks
	quitCh         chan bool
	quitChProcess  chan bool
	blocks         chan *consensus.BlockSource
	blocksAcked    chan types.Hash
}

func NewDPoS(cfg *config.Config, accounts []*types.Account, eb event.EventBus) *DPoS {
	acTrx := NewActiveTrx()
	l := ledger.NewLedger(cfg.LedgerDir())

	dps := &DPoS{
		ledger:         l,
		acTrx:          acTrx,
		accounts:       accounts,
		logger:         log.NewLogger("dpos"),
		cfg:            cfg,
		eb:             eb,
		lv:             process.NewLedgerVerifier(l),
		uncheckedCache: gcache.New(uncheckedCacheSize).LRU().Expiration(uncheckedTimeout).Build(),
		voteCache:      gcache.New(voteCacheSize).LRU().Expiration(voteCacheTimeout).Build(),
		quitCh:         make(chan bool, 1),
		quitChProcess:  make(chan bool, 1),
		blocks:         make(chan *consensus.BlockSource, maxBlocks),
		blocksAcked:    make(chan types.Hash, maxBlocks),
	}

	dps.acTrx.SetDposService(dps)
	return dps
}

func (dps *DPoS) Init() {
	if len(dps.accounts) != 0 {
		dps.refreshAccount()
	}
}

func (dps *DPoS) Start() {
	dps.logger.Info("DPOS service started!")

	go dps.acTrx.start()
	go dps.ProcessMsgLoop()

	timerFindOnlineRep := time.NewTicker(findOnlineRepInterval)
	timerRefreshPri := time.NewTicker(refreshPriInterval)
	timerUpdateUncheckedNum := time.NewTicker(time.Minute)

	for {
		select {
		case <-dps.quitCh:
			dps.logger.Info("Stopped DPOS.")
			return
		case <-timerRefreshPri.C:
			dps.logger.Info("refresh pri info.")
			go dps.refreshAccount()
		case <-timerUpdateUncheckedNum.C: //calibration
			consensus.GlobalUncheckedBlockNum.Store(uint64(dps.uncheckedCache.Len()))
		case <-timerFindOnlineRep.C:
			dps.logger.Info("begin Find Online Representatives.")
			go func() {
				err := dps.findOnlineRepresentatives()
				if err != nil {
					dps.logger.Error(err)
				}
				dps.cleanOnlineReps()
			}()
		}
	}
}

func (dps *DPoS) Stop() {
	dps.logger.Info("DPOS service stopped!")
	dps.quitCh <- true
	dps.quitChProcess <- true
	dps.acTrx.stop()
}

func (dps *DPoS) enqueueUnchecked(hash types.Hash, depHash types.Hash, bs *consensus.BlockSource) {
	if !dps.uncheckedCache.Has(depHash) {
		consensus.GlobalUncheckedBlockNum.Inc()
		blocks := new(sync.Map)
		blocks.Store(hash, bs)

		err := dps.uncheckedCache.Set(depHash, blocks)
		if err != nil {
			dps.logger.Errorf("Gap previous set cache err for block:%s", hash)
		}
	} else {
		c, err := dps.uncheckedCache.Get(depHash)
		if err != nil {
			dps.logger.Errorf("Gap previous get cache err for block:%s", hash)
		}

		blocks := c.(*sync.Map)
		blocks.Store(hash, bs)
	}
}

func (dps *DPoS) dequeueUnchecked(hash types.Hash) {
	dps.logger.Infof("dequeue gap[%s]", hash.String())
	if !dps.uncheckedCache.Has(hash) {
		return
	}

	m, err := dps.uncheckedCache.Get(hash)
	if err != nil {
		dps.logger.Errorf("dequeue unchecked err [%s] for hash [%s]", err, hash)
		return
	}

	cm := m.(*sync.Map)
	cm.Range(func(key, value interface{}) bool {
		bs := value.(*consensus.BlockSource)
		dps.logger.Infof("dequeue gap[%s] block[%s]", hash.String(), bs.Block.GetHash().String())

		result, _ := dps.lv.BlockCheck(bs.Block)
		dps.ProcessResult(result, bs)

		if dps.isResultValid(result) {
			if bs.BlockFrom == types.Synchronized {
				dps.ConfirmBlock(bs.Block)
				return true
			}

			v, e := dps.voteCache.Get(bs.Block.GetHash())
			if e == nil {
				vc := v.(*sync.Map)
				vc.Range(func(key, value interface{}) bool {
					dps.acTrx.vote(value.(*protos.ConfirmAckBlock))
					return true
				})

				dps.voteCache.Remove(bs.Block.GetHash())
			}

			localRepAccount.Range(func(key, value interface{}) bool {
				address := key.(types.Address)
				dps.saveOnlineRep(address)

				va, err := dps.voteGenerate(bs.Block, address, value.(*types.Account))
				if err != nil {
					return true
				}

				dps.acTrx.vote(va)
				dps.eb.Publish(string(common.EventBroadcast), p2p.ConfirmAck, va)

				return true
			})
		}

		return true
	})

	r := dps.uncheckedCache.Remove(hash)
	if !r {
		dps.logger.Error("remove cache for unchecked fail")
	}

	if consensus.GlobalUncheckedBlockNum.Load() > 0 {
		consensus.GlobalUncheckedBlockNum.Dec()
	}
}

func (dps *DPoS) rollbackUnchecked(hash types.Hash) {
	if !dps.uncheckedCache.Has(hash) {
		return
	}

	m, err := dps.uncheckedCache.Get(hash)
	if err != nil {
		dps.logger.Errorf("dequeue unchecked err [%s] for hash [%s]", err, hash)
		return
	}

	cm := m.(*sync.Map)
	cm.Range(func(key, value interface{}) bool {
		bs := value.(*consensus.BlockSource)
		dps.rollbackUnchecked(bs.Block.GetHash())
		return true
	})

	r := dps.uncheckedCache.Remove(hash)
	if !r {
		dps.logger.Error("remove cache for unchecked fail")
	}

	dps.voteCache.Remove(hash)

	if consensus.GlobalUncheckedBlockNum.Load() > 0 {
		consensus.GlobalUncheckedBlockNum.Dec()
	}
}

func (dps *DPoS) ProcessMsgLoop() {
	for {
	DequeueOut:
		for {
			select {
			case hash := <-dps.blocksAcked:
				dps.dequeueUnchecked(hash)
			default:
				break DequeueOut
			}
		}

		select {
		case <-dps.quitChProcess:
			return
		case bs := <-dps.blocks:
			dps.ProcessMsgDo(bs)
		case <-time.After(1 * time.Millisecond):
			//
		}
	}
}

func (dps *DPoS) isResultValid(result process.ProcessResult) bool {
	if result == process.Progress || result == process.Old {
		return true
	} else {
		return false
	}
}

func (dps *DPoS) isResultGap(result process.ProcessResult) bool {
	if result == process.GapPrevious || result == process.GapSource {
		return true
	} else {
		return false
	}
}

func (dps *DPoS) ProcessMsg(bs *consensus.BlockSource) {
	dps.blocks <- bs
}

func (dps *DPoS) ProcessMsgDo(bs *consensus.BlockSource) {
	var result process.ProcessResult
	var err error

	//block has been checked
	if !dps.acTrx.isVoting(bs.Block) {
		result, err = dps.lv.BlockCheck(bs.Block)
		if err != nil {
			dps.logger.Infof("block[%s] check err[%s]", bs.Block.GetHash().String(), err.Error())
			return
		}

		dps.ProcessResult(result, bs)
	} else {
		result = process.Progress
	}

	hash := bs.Block.GetHash()

	switch bs.Type {
	case consensus.MsgPublishReq:
		dps.logger.Infof("dps recv publishReq block[%s]", hash)
		dps.eb.Publish(string(common.EventSendMsgToPeers), p2p.PublishReq, bs.Block, bs.MsgFrom)

		if dps.isResultValid(result) {
			dps.localRepVote(bs)
		}
	case consensus.MsgConfirmReq:
		dps.logger.Infof("dps recv confirmReq block[%s]", hash)
		dps.eb.Publish(string(common.EventSendMsgToPeers), p2p.ConfirmReq, bs.Block, bs.MsgFrom)

		if dps.isResultValid(result) {
			dps.localRepVote(bs)
		}
	case consensus.MsgConfirmAck:
		dps.logger.Infof("dps recv confirmAck block[%s]", hash)
		ack := bs.Para.(*protos.ConfirmAckBlock)
		dps.eb.Publish(string(common.EventSendMsgToPeers), p2p.ConfirmAck, ack, bs.MsgFrom)
		dps.saveOnlineRep(ack.Account)

		//cache the ack messages
		if dps.isResultGap(result) {
			if dps.voteCache.Has(hash) {
				v, err := dps.voteCache.Get(hash)
				if err != nil {
					dps.logger.Error("get vote cache err")
					return
				}

				vc := v.(*sync.Map)
				vc.Store(ack.Account, ack)
			} else {
				vc := new(sync.Map)
				vc.Store(ack.Account, ack)
				err := dps.voteCache.Set(hash, vc)
				if err != nil {
					dps.logger.Error("set vote cache err")
					return
				}
			}
		} else if result == process.Progress {
			dps.acTrx.vote(ack)
			dps.localRepVote(bs)
		} else if result == process.Old {
			dps.acTrx.vote(ack)
		}
	case consensus.MsgSync:
		if result == process.Progress {
			dps.ConfirmBlock(bs.Block)
		}
	case consensus.MsgGenerateBlock:
		dps.acTrx.updatePerfTime(hash, time.Now().UnixNano(), false)
		dps.acTrx.addToRoots(bs.Block)

		if dps.isResultValid(result) {
			dps.localRepVote(bs)
		}
	default:
		//
	}
}

func (dps *DPoS) localRepVote(bs *consensus.BlockSource) {
	localRepAccount.Range(func(key, value interface{}) bool {
		address := key.(types.Address)
		dps.saveOnlineRep(address)

		va, err := dps.voteGenerate(bs.Block, address, value.(*types.Account))
		if err != nil {
			return true
		}

		dps.acTrx.vote(va)
		dps.eb.Publish(string(common.EventBroadcast), p2p.ConfirmAck, va)
		return true
	})
}

func (dps *DPoS) ConfirmBlock(blk *types.StateBlock) {
	hash := blk.GetHash()
	vk := getVoteKey(blk)

	if v, ok := dps.acTrx.roots.Load(vk); ok {
		el := v.(*Election)
		dps.acTrx.roots.Delete(el.vote.id)
		dps.acTrx.updatePerfTime(hash, time.Now().UnixNano(), true)

		if el.status.winner.GetHash().String() != hash.String() {
			dps.logger.Infof("hash:%s ...is loser", el.status.winner.GetHash().String())
			el.status.loser = append(el.status.loser, el.status.winner)
		}

		el.status.winner = blk
		el.confirmed = true

		t := el.tally()
		for _, value := range t {
			if value.block.GetHash().String() != hash.String() {
				el.status.loser = append(el.status.loser, value.block)
			}
		}

		dps.acTrx.rollBack(el.status.loser)
		dps.acTrx.addWinner2Ledger(blk)
		dps.blocksAcked <- blk.GetHash()
		dps.eb.Publish(string(common.EventConfirmedBlock), blk)
	} else {
		dps.acTrx.addWinner2Ledger(blk)
		dps.blocksAcked <- blk.GetHash()
		dps.eb.Publish(string(common.EventConfirmedBlock), blk)
	}
}

func (dps *DPoS) ProcessResult(result process.ProcessResult, bs *consensus.BlockSource) {
	blk := bs.Block
	hash := blk.GetHash()

	switch result {
	case process.Progress:
		if bs.BlockFrom == types.Synchronized {
			dps.logger.Infof("Block %s from sync,no need consensus", hash)
			now := time.Now()
			dps.eb.Publish(string(common.EventSyncing), now)
		} else if bs.BlockFrom == types.UnSynchronized {
			dps.logger.Infof("Block %s basic info is correct,begin add it to roots", hash)
			dps.acTrx.addToRoots(blk)
		} else {
			dps.logger.Errorf("Block %s UnKnow from", hash)
		}
	case process.BadSignature:
		dps.logger.Errorf("Bad signature for block: %s", hash)
	case process.BadWork:
		dps.logger.Errorf("Bad work for block: %s", hash)
	case process.BalanceMismatch:
		dps.logger.Errorf("Balance mismatch for block: %s", hash)
	case process.Old:
		dps.logger.Debugf("Old for block: %s", hash)
	case process.UnReceivable:
		dps.logger.Errorf("UnReceivable for block: %s", hash)
	case process.GapSmartContract:
		dps.logger.Errorf("GapSmartContract for block: %s", hash)
		//dps.processGapSmartContract(blk)
	case process.InvalidData:
		dps.logger.Errorf("InvalidData for block: %s", hash)
	case process.Other:
		dps.logger.Errorf("UnKnow process result for block: %s", hash)
	case process.Fork:
		dps.logger.Errorf("Fork for block: %s", hash)
		dps.ProcessFork(blk)
	case process.GapPrevious:
		dps.logger.Infof("block:[%s] Gap previous:[%s]", hash, blk.Previous.String())
		dps.enqueueUnchecked(hash, blk.Previous, bs)
	case process.GapSource:
		dps.logger.Infof("block:[%s] Gap source:[%s]", hash, blk.Link.String())
		dps.enqueueUnchecked(hash, blk.Link, bs)
	}
}

func (dps *DPoS) ProcessFork(newBlock *types.StateBlock) {
	confirmedBlock := dps.findAnotherForkedBlock(newBlock)

	if dps.acTrx.addToRoots(confirmedBlock) {
		localRepAccount.Range(func(key, value interface{}) bool {
			address := key.(types.Address)
			dps.saveOnlineRep(address)

			va, err := dps.voteGenerateWithSeq(confirmedBlock, address, value.(*types.Account))
			if err != nil {
				return true
			}

			dps.acTrx.vote(va)
			dps.eb.Publish(string(common.EventBroadcast), p2p.ConfirmAck, va)
			return true
		})
		dps.eb.Publish(string(common.EventBroadcast), p2p.ConfirmReq, confirmedBlock)
	}
}

func (dps *DPoS) findAnotherForkedBlock(block *types.StateBlock) *types.StateBlock {
	hash := block.Parent()

	forkedHash, err := dps.ledger.GetChild(hash, block.Address)
	if err != nil {
		dps.logger.Error(err)
		return block
	}

	forkedBlock, err := dps.ledger.GetStateBlock(forkedHash)
	if err != nil {
		dps.logger.Error(err)
		return block
	}

	return forkedBlock
}

func (dps *DPoS) voteGenerate(block *types.StateBlock, account types.Address, acc *types.Account) (*protos.ConfirmAckBlock, error) {
	va := &protos.ConfirmAckBlock{
		Sequence:  0,
		Blk:       block,
		Account:   account,
		Signature: acc.Sign(block.GetHash()),
	}
	return va, nil
}

func (dps *DPoS) voteGenerateWithSeq(block *types.StateBlock, account types.Address, acc *types.Account) (*protos.ConfirmAckBlock, error) {
	va := &protos.ConfirmAckBlock{
		Sequence:  uint32(time.Now().Unix()),
		Blk:       block,
		Account:   account,
		Signature: acc.Sign(block.GetHash()),
	}
	return va, nil
}

func (dps *DPoS) refreshAccount() {
	var b bool
	var addr types.Address

	for _, v := range dps.accounts {
		addr = v.Address()
		b = dps.isRepresentation(addr)
		if b {
			localRepAccount.Store(addr, v)
		}
	}

	var count uint32
	localRepAccount.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	dps.logger.Infof("there is %d local reps", count)
	if count > 1 {
		dps.logger.Error("it is very dangerous to run two or more representatives on one node")
	}
}

func (dps *DPoS) isRepresentation(address types.Address) bool {
	if _, err := dps.ledger.GetRepresentation(address); err != nil {
		return false
	}
	return true
}

func (dps *DPoS) saveOnlineRep(addr types.Address) {
	now := time.Now().Add(repTimeout).UTC().Unix()
	dps.onlineReps.Store(addr, now)
}

func (dps *DPoS) GetOnlineRepresentatives() []types.Address {
	var repAddresses []types.Address

	dps.onlineReps.Range(func(key, value interface{}) bool {
		addr := key.(types.Address)
		repAddresses = append(repAddresses, addr)
		return true
	})

	return repAddresses
}

func (dps *DPoS) findOnlineRepresentatives() error {
	blk, err := dps.ledger.GetRandomStateBlock()
	if err != nil {
		return err
	}

	localRepAccount.Range(func(key, value interface{}) bool {
		address := key.(types.Address)
		dps.saveOnlineRep(address)

		va, err := dps.voteGenerateWithSeq(blk, address, value.(*types.Account))
		if err != nil {
			return true
		}

		dps.acTrx.vote(va)
		dps.eb.Publish(string(common.EventBroadcast), p2p.ConfirmAck, va)

		return true
	})

	return nil
}

func (dps *DPoS) cleanOnlineReps() {
	var repAddresses []*types.Address
	now := time.Now().UTC().Unix()

	dps.onlineReps.Range(func(key, value interface{}) bool {
		addr := key.(types.Address)
		v := value.(int64)
		if v < now {
			dps.onlineReps.Delete(addr)
		} else {
			repAddresses = append(repAddresses, &addr)
		}
		return true
	})

	_ = dps.ledger.SetOnlineRepresentations(repAddresses)
}

func (dps *DPoS) calculateAckHash(va *protos.ConfirmAckBlock) (types.Hash, error) {
	data, err := protos.ConfirmAckBlockToProto(va)
	if err != nil {
		return types.ZeroHash, err
	}

	version := dps.cfg.Version
	message := p2p.NewQlcMessage(data, byte(version), p2p.ConfirmAck)
	hash, err := types.HashBytes(message)
	if err != nil {
		return types.ZeroHash, err
	}

	return hash, nil
}