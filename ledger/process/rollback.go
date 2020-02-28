package process

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/yireyun/go-queue"

	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/storage"
	"github.com/qlcchain/go-qlc/common/topic"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/trie"
	"github.com/qlcchain/go-qlc/vm/contract"
	cabi "github.com/qlcchain/go-qlc/vm/contract/abi"
	"github.com/qlcchain/go-qlc/vm/vmstore"
)

func (lv *LedgerVerifier) Rollback(hash types.Hash) error {
	if b, err := lv.l.HasBlockCache(hash); b && err == nil {
		return lv.RollbackCache(hash)
	}

	if b, err := lv.l.HasStateBlockConfirmed(hash); !b || err != nil {
		lv.logger.Warnf("rollback block not found: %s", hash.String())
		return nil
	}
	lv.logger.Warnf("process rollback  block: %s", hash.String())
	// get blocks to roll back
	rollbackMap := make(map[types.Hash]*types.StateBlock)
	relatedBlocks := queue.NewQueue(512)
	relatedBlocks.Put(hash)
	lv.logger.Debugf("put block to queue %s ", hash.String())

	for {
		if v, ok, _ := relatedBlocks.Get(); ok {
			// get oldest block
			oldestHash := v.(types.Hash)
			oldestBlock, err := lv.l.GetStateBlockConfirmed(oldestHash)
			if err != nil {
				return fmt.Errorf("can not get block  %s", oldestHash.String())
			}
			lv.logger.Debugf("get block from  queue %s (%s) ,%s  ", oldestBlock.GetHash().String(), oldestBlock.GetType().String(), oldestBlock.Address.String())

			// put oldest block to rollbackMap
			h, err := types.HashBytes(oldestBlock.Address[:], oldestBlock.Token[:])
			if err != nil {
				return fmt.Errorf("get hash key error %s", err)
			}
			if rBlock, ok := rollbackMap[h]; ok {
				lv.logger.Debugf("get %s from rollback of %s ", rBlock.GetHash().String(), oldestBlock.GetAddress().String())
				if t, err := lv.blockOrderCompare(oldestBlock, rBlock); t && err == nil {
					lv.logger.Debugf("put block to rollback %s (%s), %s ", oldestBlock.GetHash().String(), oldestBlock.GetType().String(), oldestBlock.Address.String())
					rollbackMap[h] = oldestBlock
				} else if err != nil {
					return err
				}
			} else {
				lv.logger.Debugf("put block  to rollback %s (%s), %s ", oldestBlock.GetHash().String(), oldestBlock.GetType().String(), oldestBlock.Address.String())
				rollbackMap[h] = oldestBlock
			}

			// get header block
			tm, err := lv.l.GetTokenMetaConfirmed(oldestBlock.GetAddress(), oldestBlock.GetToken())
			if err != nil {
				return fmt.Errorf("can not get account of block %s", oldestHash.String())
			}
			headerHash := tm.Header
			headerBlock, err := lv.l.GetStateBlockConfirmed(headerHash)
			if err != nil {
				return fmt.Errorf("can not get header block %s", headerHash.String())
			}

			curBlock := headerBlock
			// put link block to rollbackMap
			for {
				//if curBlock.IsOpen() {
				//	break
				//}

				if curBlock.IsSendBlock() {
					linkHash, err := lv.l.GetBlockLink(curBlock.GetHash())
					// link not found is not error ,may be send block has created but receiver block has not created
					if err != nil && err != ledger.ErrLinkNotFound {
						return fmt.Errorf("can not get link hash %s", curBlock.GetHash().String())
					}
					if !linkHash.IsZero() {
						linkBlock, err := lv.l.GetStateBlockConfirmed(linkHash)
						if err != nil {
							return fmt.Errorf("can not get link block %s", linkHash.String())
						}
						ha, err := types.HashBytes(linkBlock.Address[:], linkBlock.Token[:])
						if err != nil {
							return fmt.Errorf("get hash key error %s", err)
						}
						if rBlock, ok := rollbackMap[ha]; ok {
							lv.logger.Debugf("get link %s from rollback of %s ", rBlock.GetHash().String(), linkBlock.GetAddress().String())
							if t, err := lv.blockOrderCompare(linkBlock, rBlock); t && err == nil {
								lv.logger.Debugf("put block to queue %s (%s) ,%s ", linkBlock.GetHash().String(), linkBlock.GetType().String(), linkBlock.Address.String())
								relatedBlocks.Put(linkBlock.GetHash())
							} else if err != nil {
								return err
							}
						} else {
							lv.logger.Debugf("put block to queue %s (%s), %s ", linkBlock.GetHash().String(), linkBlock.GetType().String(), linkBlock.Address.String())
							relatedBlocks.Put(linkBlock.GetHash())
						}
					}
				}

				if curBlock.GetHash() == oldestHash {
					break
				}

				curHash := curBlock.GetPrevious()
				curBlock, err = lv.l.GetStateBlockConfirmed(curHash)
				if err != nil {
					return fmt.Errorf("can not get previous block %s", curHash.String())
				}
			}
		} else {
			break
		}
	}

	return lv.l.Cache().BatchUpdate(func(c *ledger.Cache) error {
		batch := lv.l.DBStore().Batch(true)
		if err := lv.rollbackBlocks(rollbackMap, c, batch); err != nil {
			lv.logger.Error(err)
			return err
		}
		return lv.l.DBStore().PutBatch(batch)
	})
}

func (lv *LedgerVerifier) RollbackCache(hash types.Hash) error {
	if b, err := lv.l.HasBlockCache(hash); b && err == nil {
		lv.logger.Warnf("process rollback cache block: %s", hash.String())
		err := lv.l.DBStore().BatchWrite(true, func(batch storage.Batch) error {
			if err := lv.rollbackCache(hash, batch); err != nil {
				lv.logger.Error(err)
				return err
			}
			return nil
		})
		if err != nil {
			lv.logger.Error(err)
			return err
		}
		return nil
	}
	return nil
}

// rollback cache blocks
func (lv *LedgerVerifier) rollbackCache(hash types.Hash, batch storage.Batch) error {
	block, err := lv.l.GetBlockCache(hash)
	if err != nil {
		return fmt.Errorf("get cache block (%s) err: %s", hash.String(), err)
	}

	// get all blocks of the address
	blocks := make([]*types.StateBlock, 0)
	err = lv.l.GetBlockCaches(func(b *types.StateBlock) error {
		if block.GetAddress() == b.GetAddress() && block.GetToken() == b.GetToken() {
			blocks = append(blocks, b)
		}
		return nil
	})
	if err != nil {
		lv.logger.Error("get block cache error")
	}

	// if receive repeated , rollback later block
	rollBlock := block
	if rollBlock.IsReceiveBlock() {
		for _, b := range blocks {
			if rollBlock.GetLink() == b.GetLink() && rollBlock.GetHash() != b.GetHash() {
				if b.Timestamp > rollBlock.Timestamp {
					rollBlock = b
				}
				break
			}
		}
	}

	// get rollback blocks
	tempBlocks := make([]*types.StateBlock, 0)
	for _, b := range blocks {
		if b.Timestamp >= rollBlock.Timestamp {
			tempBlocks = append(tempBlocks, b)
		}
	}

	// sort
	sort.Slice(tempBlocks, func(i, j int) bool {
		return tempBlocks[i].Timestamp < tempBlocks[j].Timestamp
	})

	rollBlocks := make([]*types.StateBlock, 0)
	rollBlocks = append(rollBlocks, rollBlock)
	for {
		findPre := false
		for _, b := range tempBlocks {
			if b.GetPrevious() == rollBlock.GetHash() {
				rollBlocks = append(rollBlocks, b)
				rollBlock = b
				findPre = true
				break
			}
		}
		if !findPre {
			break
		}
	}

	// delete blocks
	if err := lv.rollbackCacheBlocks(rollBlocks, true, batch); err != nil {
		lv.logger.Error(err)
		return err
	}
	return nil
}

func (lv *LedgerVerifier) rollbackCacheBlocks(blocks []*types.StateBlock, cache bool, batch storage.Batch) error {
	if len(blocks) == 0 {
		return nil
	}
	if cache {
		for i := len(blocks) - 1; i >= 0; i-- {
			block := blocks[i]

			if err := lv.l.DeleteBlockCache(block.GetHash(), batch); err != nil {
				return fmt.Errorf("delete BlockCache fail(%s), hash(%s)", err, block.GetHash().String())
			}
			lv.l.EB.Publish(topic.EventRollback, block.GetHash())
			lv.logger.Infof("rollback delete cache block %s (previous: %s, type: %s,  address: %s)", block.GetHash().String(), block.GetPrevious().String(), block.GetType(), block.GetAddress().String())

			if b, _ := lv.l.HasBlockCache(block.GetPrevious()); b {
				if err := lv.rollbackCacheAccount(block, batch); err != nil {
					lv.logger.Errorf("roll back cache account error : %s", err)
					return err
				}
			} else {
				if err := lv.rollbackCacheAccountDel(block.GetAddress(), block.GetToken(), batch); err != nil {
					lv.logger.Errorf("roll back cache account del error : %s", err)
					return err
				}
			}
		}
		return nil
	}
	for _, block := range blocks {

		if err := lv.l.DeleteBlockCache(block.GetHash(), batch); err != nil {
			return fmt.Errorf("delete BlockCache fail(%s), hash(%s)", err, block.GetHash().String())
		}
		lv.l.EB.Publish(topic.EventRollback, block.GetHash())
		lv.logger.Errorf("rollback delete cache block %s (previous: %s, type: %s,  address: %s)", block.GetHash().String(), block.GetPrevious().String(), block.GetType(), block.GetAddress().String())
	}

	blk := blocks[0]
	if err := lv.rollbackCacheAccountDel(blk.GetAddress(), blk.GetToken(), batch); err != nil {
		lv.logger.Warnf("roll back cache account error : %s", err)
		return err
	}
	return nil
}

func (lv *LedgerVerifier) rollbackCacheAccount(block *types.StateBlock, batch storage.Batch) error {
	ac, err := lv.l.GetAccountMeteCache(block.GetAddress(), batch)
	if err == nil {
		preBlk, err := lv.l.GetBlockCache(block.GetPrevious())
		if err == nil {
			if preBlk.GetToken() == common.ChainToken() {
				ac.CoinVote = preBlk.GetVote()
				ac.CoinOracle = preBlk.GetOracle()
				ac.CoinNetwork = preBlk.GetNetwork()
				ac.CoinStorage = preBlk.GetStorage()
				ac.CoinBalance = preBlk.GetBalance()
			}
			tm := ac.Token(block.GetToken())
			if tm != nil {
				tm.Balance = preBlk.GetBalance()
				tm.Header = preBlk.GetHash()
				tm.Representative = preBlk.GetRepresentative()
				tm.BlockCount = tm.BlockCount - 1
				tm.Modified = common.TimeNow().Unix()
				lv.logger.Debug("update token, ", tm)
				for index, t := range ac.Tokens {
					if t.Type == tm.Type {
						ac.Tokens[index] = tm
						if err := lv.l.UpdateAccountMeteCache(ac, batch); err != nil {
							return err
						}
						lv.logger.Warnf("rollback update account cache, %s", ac.String())
						return nil
					}
				}
			}
		}
	}
	return nil
}

func (lv *LedgerVerifier) rollbackCacheAccountDel(address types.Address, token types.Hash, batch storage.Batch) error {
	ac, err := lv.l.GetAccountMeteCache(address, batch)
	if err != nil {
		if err == ledger.ErrAccountNotFound {
			return nil
		}
		return err
	}

	if tm := ac.Token(token); tm == nil {
		return nil
	} else {
		if len(ac.Tokens) == 1 {
			if err := lv.l.DeleteAccountMetaCache(address, batch); err != nil {
				return err
			}
			lv.logger.Infof("rollback delete account cache, %s", address.String())
			return nil
		} else {
			if err := lv.l.DeleteTokenMetaCache(address, token, batch); err != nil {
				return err
			}
			lv.logger.Infof("rollback delete token cache, %s, %s", address, token)
			return nil
		}
	}
}

// rollback confirmed blocks
func (lv *LedgerVerifier) rollbackBlocks(rollbackMap map[types.Hash]*types.StateBlock, cache *ledger.Cache, batch storage.Batch) error {
	sendBlocks, err := lv.sendBlocksInRollback(rollbackMap)
	if err != nil {
		return err
	}

	for _, oldestBlock := range rollbackMap {
		tm, err := lv.l.GetTokenMetaConfirmed(oldestBlock.GetAddress(), oldestBlock.GetToken())
		if err != nil {
			return fmt.Errorf("get tokenmeta error: %s (%s)", err, oldestBlock.GetHash().String())
		}
		headerHash := tm.Header

		hashCur := headerHash
		blockCur, err := lv.l.GetStateBlockConfirmed(hashCur)
		if err != nil {
			return fmt.Errorf("get block error: %s (%s)", err, hashCur.String())
		}
		lv.logger.Debug("--- start rollback chain --- ", oldestBlock.GetHash())

		for {
			blockType := blockCur.GetType()
			blockPre := new(types.StateBlock)
			if !blockCur.IsOpen() {
				blockPre, err = lv.l.GetStateBlockConfirmed(blockCur.Previous)
				if err != nil {
					return fmt.Errorf("get previous block %s : %s", blockCur.Previous.String(), err)
				}
			}
			lv.logger.Debug("--- start rollback ", blockCur.GetHash())
			switch blockType {
			case types.Open:
				if err := lv.rollBackTokenDel(tm, cache); err != nil {
					return fmt.Errorf("rollback token fail(%s), open(%s)", err, hashCur)
				}
				if err := lv.rollBackRep(blockCur.GetRepresentative(), blockCur, nil, false, blockCur.GetToken(), cache); err != nil {
					return fmt.Errorf("rollback representative fail(%s), open(%s)", err, hashCur)
				}
				if err := lv.rollBackFrontier(types.Hash{}, blockCur.GetHash(), cache); err != nil {
					return fmt.Errorf("rollback frontier fail(%s), open(%s)", err, hashCur)
				}
				if _, ok := sendBlocks[blockCur.GetLink()]; !ok {
					if err := lv.rollBackPendingAdd(blockCur, tm.Balance, blockCur.GetToken(), cache); err != nil {
						return fmt.Errorf("rollback pending fail(%s), open(%s)", err, hashCur)
					}
				}
			case types.Send:
				if err := lv.rollBackToken(tm, blockPre, cache); err != nil {
					return fmt.Errorf("rollback token fail(%s), send(%s)", err, hashCur)
				}
				if err := lv.rollBackFrontier(blockPre.GetHash(), blockCur.GetHash(), cache); err != nil {
					return fmt.Errorf("rollback frontier fail(%s), send(%s)", err, hashCur)
				}
				if err := lv.rollBackRep(blockCur.GetRepresentative(), blockCur, blockPre, true, blockCur.GetToken(), cache); err != nil {
					return fmt.Errorf("rollback representative fail(%s), send(%s)", err, hashCur)
				}
				if err := lv.rollBackPendingDel(blockCur, cache); err != nil {
					return fmt.Errorf("rollback pending fail(%s), send(%s)", err, hashCur)
				}
			case types.Receive:
				if err := lv.rollBackToken(tm, blockPre, cache); err != nil {
					return fmt.Errorf("rollback token fail(%s), receive(%s)", err, hashCur)
				}
				if err := lv.rollBackFrontier(blockPre.GetHash(), blockCur.GetHash(), cache); err != nil {
					return fmt.Errorf("rollback frontier fail(%s), receive(%s)", err, hashCur)
				}
				if err := lv.rollBackRep(blockCur.GetRepresentative(), blockCur, blockPre, false, blockCur.GetToken(), cache); err != nil {
					return fmt.Errorf("rollback representative fail(%s), receive(%s)", err, hashCur)
				}
				if _, ok := sendBlocks[blockCur.GetLink()]; !ok {
					if err := lv.rollBackPendingAdd(blockCur, blockCur.GetBalance().Sub(blockPre.GetBalance()), blockCur.GetToken(), cache); err != nil {
						return fmt.Errorf("rollback pending fail(%s), receive(%s)", err, hashCur)
					}
				}
			case types.Change, types.Online:
				if err := lv.rollBackToken(tm, blockPre, cache); err != nil {
					return fmt.Errorf("rollback token fail(%s), change(%s)", err, hashCur)
				}
				if err := lv.rollBackFrontier(blockPre.GetHash(), blockCur.GetHash(), cache); err != nil {
					return fmt.Errorf("rollback frontier fail(%s), change(%s)", err, hashCur)
				}
				if err := lv.rollBackRepChange(blockPre.GetRepresentative(), blockCur.GetRepresentative(), blockCur, cache); err != nil {
					return fmt.Errorf("rollback representative fail(%s), change(%s)", err, hashCur)
				}
			case types.ContractReward:
				previousHash := blockCur.GetPrevious()
				if previousHash.IsZero() {
					if err := lv.rollBackTokenDel(tm, cache); err != nil {
						return fmt.Errorf("rollback token fail(%s), ContractReward(%s)", err, hashCur)
					}
					if err := lv.rollBackFrontier(types.Hash{}, blockCur.GetHash(), cache); err != nil {
						return fmt.Errorf("rollback frontier fail(%s), ContractReward(%s)", err, hashCur)
					}
				} else {
					if err := lv.rollBackToken(tm, blockPre, cache); err != nil {
						return fmt.Errorf("rollback token fail(%s), ContractReward(%s)", err, hashCur)
					}
					if err := lv.rollBackFrontier(blockPre.GetHash(), blockCur.GetHash(), cache); err != nil {
						return fmt.Errorf("rollback frontier fail(%s), ContractReward(%s)", err, hashCur)
					}
				}
				if _, ok := sendBlocks[blockCur.GetLink()]; !ok {
					if err := lv.rollBackPendingAdd(blockCur, types.ZeroBalance, types.ZeroHash, cache); err != nil {
						return fmt.Errorf("rollback pending fail(%s), ContractReward(%s)", err, hashCur)
					}
				}
				if err := lv.rollBackContractData(blockCur, cache); err != nil {
					return fmt.Errorf("rollback contract data fail(%s), ContractReward(%s)", err, blockCur.GetHash().String())
				}
			case types.ContractSend:
				if err := lv.rollBackToken(tm, blockPre, cache); err != nil {
					return fmt.Errorf("rollback token fail(%s), ContractSend(%s)", err, hashCur)
				}
				if err := lv.rollBackFrontier(blockPre.GetHash(), blockCur.GetHash(), cache); err != nil {
					return fmt.Errorf("rollback frontier fail(%s), ContractSend(%s)", err, hashCur)
				}
				if err := lv.rollBackPendingDel(blockCur, cache); err != nil {
					return fmt.Errorf("rollback pending fail(%s), ContractSend(%s)", err, hashCur)
				}
				if err := lv.rollBackContractData(blockCur, cache); err != nil {
					return fmt.Errorf("rollback contract data fail(%s), ContractSend(%s)", err, blockCur.String())
				}
			}

			// rollback Block
			if err := lv.l.DeleteStateBlock(hashCur, cache); err != nil {
				return fmt.Errorf("delete state block error: %s, %s", err, hashCur)
			}
			lv.l.EB.Publish(topic.EventRollback, hashCur)
			lv.logger.Warnf("rollback delete block done: %s (previous: %s, type: %s,  address: %s) ", hashCur.String(), blockCur.GetPrevious().String(), blockCur.GetType(), blockCur.GetAddress().String())

			if err := lv.checkBlockUnConfirmed(blockCur, batch); err != nil {
				lv.logger.Errorf("roll back block cache error : %s", err)
				return err
			}

			if hashCur == oldestBlock.GetHash() {
				break
			}

			hashCur = blockCur.GetPrevious()
			blockCur, err = lv.l.GetStateBlockConfirmed(hashCur)
			if err != nil {
				return fmt.Errorf("get previous block error %s, %s ", err, hashCur.String())
			}
		}
	}
	return nil
}

func (lv *LedgerVerifier) checkBlockUnConfirmed(block *types.StateBlock, batch storage.Batch) error {
	rollbacks := make([]*types.StateBlock, 0)
	err := lv.l.GetBlockCaches(func(b *types.StateBlock) error {
		if block.GetAddress() == b.GetAddress() && block.GetToken() == b.GetToken() {
			rollbacks = append(rollbacks, b)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(rollbacks) > 0 {
		if err = lv.rollbackCacheBlocks(rollbacks, false, batch); err != nil {
			return err
		}
	} else {
		// maybe no unconfirmed block ,but unconfirmed account exist
		if err := lv.rollbackCacheAccountDel(block.GetAddress(), block.GetToken(), batch); err != nil {
			lv.logger.Errorf("roll back account cache error : %s", err)
			return err
		}
	}
	if block.IsSendBlock() {
		err := lv.l.GetBlockCaches(func(b *types.StateBlock) error {
			if block.GetHash() == b.GetLink() {
				err = lv.rollbackCache(b.GetHash(), batch)
				if err != nil {
					lv.logger.Error(err)
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

//func (lv *LedgerVerifier) rollBackPendingKind(block *types.StateBlock, txn db.StoreTxn) error {
//	if block.IsReceiveBlock() {
//		pendingKey := &types.PendingKey{
//			Address: block.GetAddress(),
//			Hash:    block.GetLink(),
//		}
//		if err := lv.l.UpdatePending(pendingKey, types.PendingNotUsed, txn); err != nil {
//			return err
//		}
//	}
//	return nil
//}

// all Send block to rollback
func (lv *LedgerVerifier) sendBlocksInRollback(blocks map[types.Hash]*types.StateBlock) (map[types.Hash]types.Address, error) {
	sendBlocks := make(map[types.Hash]types.Address)
	for _, oldestBlock := range blocks {
		tm, err := lv.l.GetTokenMetaConfirmed(oldestBlock.GetAddress(), oldestBlock.GetToken())
		if err != nil {
			return nil, fmt.Errorf("get tokenmeta error: %s (%s)", err, oldestBlock.GetHash().String())
		}

		curHash := tm.Header
		curBlock, err := lv.l.GetStateBlockConfirmed(curHash)
		if err != nil {
			return nil, fmt.Errorf("get block error: %s (%s)", err, curHash.String())
		}
		for {
			if curBlock.IsSendBlock() {
				sendBlocks[curBlock.GetHash()] = curBlock.GetAddress()
			}

			if curBlock.GetHash() == oldestBlock.GetHash() {
				break
			}

			curHash := curBlock.GetPrevious()
			curBlock, err = lv.l.GetStateBlockConfirmed(curHash)
			if err != nil {
				return nil, fmt.Errorf("can not get previous block %s", curHash.String())
			}
		}
	}
	return sendBlocks, nil
}

// if aBlock is created early than bBlock , return true, else return false
func (lv *LedgerVerifier) blockOrderCompare(aBlock, bBlock *types.StateBlock) (bool, error) {
	lv.logger.Debugf("block compare, %s %s %s ", aBlock.GetHash().String(), bBlock.GetHash().String(), aBlock.Address.String())
	if aBlock.GetAddress() != bBlock.GetAddress() || aBlock.GetToken() != bBlock.GetToken() {
		return false, fmt.Errorf("can not compare block, %s %s", aBlock.GetHash().String(), bBlock.GetHash().String())
	}
	tm, _ := lv.l.GetTokenMetaConfirmed(aBlock.GetAddress(), aBlock.GetToken())
	curHash := tm.Header
	for {
		curBlock, err := lv.l.GetStateBlockConfirmed(curHash)
		if err != nil {
			return false, fmt.Errorf("get block error %s", curBlock.String())
		}
		if curBlock.GetHash() == aBlock.GetHash() {
			return false, nil
		}
		if curBlock.GetHash() == bBlock.GetHash() {
			return true, nil
		}
		curHash = curBlock.GetPrevious()
		if curHash.IsZero() {
			return false, fmt.Errorf("can not find blocks when block compare")
		}
	}
}

func (lv *LedgerVerifier) rollBackFrontier(pre types.Hash, cur types.Hash, cache *ledger.Cache) error {
	frontier, err := lv.l.GetFrontier(cur, cache)
	if err != nil {
		return err
	}
	lv.logger.Debug("delete frontier, ", frontier)
	if err := lv.l.DeleteFrontier(cur, cache); err != nil {
		return err
	}
	if !pre.IsZero() {
		frontier.HeaderBlock = pre
		lv.logger.Debug("add frontier, ", frontier)
		if err := lv.l.AddFrontier(frontier, cache); err != nil {
			return err
		}
	}
	return nil
}

func (lv *LedgerVerifier) rollBackToken(token *types.TokenMeta, pre *types.StateBlock, cache *ledger.Cache) error {
	ac, err := lv.l.GetAccountMetaConfirmed(token.BelongTo, cache)
	if err != nil {
		return err
	}
	if pre.GetToken() == common.ChainToken() {
		ac.CoinVote = pre.GetVote()
		ac.CoinOracle = pre.GetOracle()
		ac.CoinNetwork = pre.GetNetwork()
		ac.CoinStorage = pre.GetStorage()
		ac.CoinBalance = pre.GetBalance()
	}
	tm := ac.Token(pre.GetToken())
	if tm == nil {
		return fmt.Errorf("can not get token %s from account %s", pre.GetToken().String(), ac.Address.String())
	}
	tm.Balance = pre.GetBalance()
	tm.Header = pre.GetHash()
	tm.Representative = pre.GetRepresentative()
	tm.BlockCount = tm.BlockCount - 1
	tm.Modified = common.TimeNow().Unix()
	lv.logger.Debug("update token, ", tm)
	for index, t := range ac.Tokens {
		if t.Type == tm.Type {
			ac.Tokens[index] = tm
			if err := lv.l.UpdateAccountMeta(ac, cache); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

func (lv *LedgerVerifier) rollBackTokenDel(tm *types.TokenMeta, cache *ledger.Cache) error {
	address := tm.BelongTo
	lv.logger.Debug("delete token, ", address, tm.Type)
	if err := lv.l.DeleteTokenMetaConfirmed(address, tm.Type, cache); err != nil {
		return err
	}
	ac, err := lv.l.GetAccountMetaConfirmed(address, cache)
	if err != nil {
		return err
	}
	if len(ac.Tokens) == 0 {
		if err := lv.l.DeleteAccountMeta(address, cache); err != nil {
			return err
		}
	}
	return nil
}

func (lv *LedgerVerifier) rollBackRep(representative types.Address, blockCur, blockPre *types.StateBlock, isSend bool, token types.Hash, cache *ledger.Cache) error {
	if token == common.ChainToken() {
		if isSend {
			diff := &types.Benefit{
				Vote:    blockPre.GetVote().Sub(blockCur.GetVote()),
				Network: blockPre.GetNetwork().Sub(blockCur.GetNetwork()),
				Oracle:  blockPre.GetOracle().Sub(blockCur.GetOracle()),
				Storage: blockPre.GetStorage().Sub(blockCur.GetStorage()),
				Balance: blockPre.GetBalance().Sub(blockCur.GetBalance()),
				Total:   blockPre.TotalBalance().Sub(blockCur.TotalBalance()),
			}
			lv.logger.Debugf("add rep(%s) to %s", diff, representative)
			if err := lv.l.AddRepresentation(representative, diff, cache); err != nil {
				return err
			}
		} else {
			diff := new(types.Benefit)
			if blockPre == nil {
				diff = &types.Benefit{
					Vote:    blockCur.GetVote(),
					Network: blockCur.GetNetwork(),
					Oracle:  blockCur.GetOracle(),
					Storage: blockCur.GetStorage(),
					Balance: blockCur.GetBalance(),
					Total:   blockCur.TotalBalance(),
				}
			} else {
				diff = &types.Benefit{
					Vote:    blockCur.GetVote().Sub(blockPre.GetVote()),
					Network: blockCur.GetNetwork().Sub(blockPre.GetNetwork()),
					Oracle:  blockCur.GetOracle().Sub(blockPre.GetOracle()),
					Storage: blockCur.GetStorage().Sub(blockPre.GetStorage()),
					Balance: blockCur.GetBalance().Sub(blockPre.GetBalance()),
					Total:   blockCur.TotalBalance().Sub(blockPre.TotalBalance()),
				}
			}
			lv.logger.Debugf("sub rep %s from %s", diff, representative)
			if err := lv.l.SubRepresentation(representative, diff, cache); err != nil {
				return err
			}
		}
	}
	return nil
}

func (lv *LedgerVerifier) rollBackRepChange(preRepresentation types.Address, curRepresentation types.Address, blockCur *types.StateBlock, cache *ledger.Cache) error {
	diff := &types.Benefit{
		Vote:    blockCur.GetVote(),
		Network: blockCur.GetNetwork(),
		Oracle:  blockCur.GetOracle(),
		Storage: blockCur.GetStorage(),
		Balance: blockCur.GetBalance(),
		Total:   blockCur.TotalBalance(),
	}
	lv.logger.Debugf("add rep(%s) to %s", diff, preRepresentation)
	if err := lv.l.AddRepresentation(preRepresentation, diff, cache); err != nil {
		return err
	}
	lv.logger.Debugf("sub rep(%s) from %s", diff, curRepresentation)
	if err := lv.l.SubRepresentation(curRepresentation, diff, cache); err != nil {
		return err
	}
	return nil
}

func (lv *LedgerVerifier) rollBackPendingAdd(blockCur *types.StateBlock, amount types.Balance, token types.Hash, cache *ledger.Cache) error {
	blockLink, err := lv.l.GetStateBlockConfirmed(blockCur.GetLink())
	if err != nil {
		return fmt.Errorf("%s %s", err, blockCur.GetLink())
	}

	if blockCur.GetType() == types.ContractReward {
		if c, ok, err := contract.GetChainContract(types.Address(blockLink.Link), blockLink.Data); ok && err == nil {
			switch c.GetDescribe().GetVersion() {
			case contract.SpecVer1:
				if pendingKey, pendingInfo, err := c.DoPending(blockLink); err == nil && pendingKey != nil {
					lv.logger.Debug("add contract reward pending , ", pendingKey)
					if err := lv.l.AddPending(pendingKey, pendingInfo, cache); err != nil {
						return err
					}
				}
			case contract.SpecVer2:
				vmCtx := vmstore.NewVMContext(lv.l)
				if pendingKey, pendingInfo, err := c.ProcessSend(vmCtx, blockLink); err == nil && pendingKey != nil {
					lv.logger.Debug("contractSend add pending , ", pendingKey)
					if err := lv.l.AddPending(pendingKey, pendingInfo, cache); err != nil {
						return err
					}
				} else {
					return fmt.Errorf("process send error, %s", err)
				}
			default:
				return fmt.Errorf("unsupported chain contract version %d", c.GetDescribe().GetVersion())
			}
		}
		return nil
	} else {
		pendingkey := types.PendingKey{
			Address: blockCur.GetAddress(),
			Hash:    blockLink.GetHash(),
		}
		pendinginfo := types.PendingInfo{
			Source: blockLink.GetAddress(),
			Amount: amount,
			Type:   token,
		}
		lv.logger.Debug("add pending, ", pendingkey, pendinginfo)
		if err := lv.l.AddPending(&pendingkey, &pendinginfo, cache); err != nil {
			return err
		}
		return nil
	}
}

func (lv *LedgerVerifier) rollBackPendingDel(blockCur *types.StateBlock, cache *ledger.Cache) error {
	if blockCur.GetType() == types.ContractSend {
		if c, ok, err := contract.GetChainContract(types.Address(blockCur.Link), blockCur.Data); ok && err == nil {
			switch c.GetDescribe().GetVersion() {
			case contract.SpecVer1:
				if pendingKey, _, err := c.DoPending(blockCur); err == nil && pendingKey != nil {
					lv.logger.Debug("delete contract send pending , ", pendingKey)
					if err := lv.l.DeletePending(pendingKey, cache); err != nil {
						return err
					}
				}
			case contract.SpecVer2:
				vmCtx := vmstore.NewVMContext(lv.l)
				if pendingKey, _, err := c.ProcessSend(vmCtx, blockCur); err == nil && pendingKey != nil {
					lv.logger.Debug("delete contract send pending , ", pendingKey)
					if err := lv.l.DeletePending(pendingKey, cache); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unsupported chain contract %d", c.GetDescribe().GetVersion())
			}
		}
		return nil
	} else {
		address := types.Address(blockCur.Link)
		hash := blockCur.GetHash()
		pendingkey := types.PendingKey{
			Address: address,
			Hash:    hash,
		}
		lv.logger.Debug("delete pending ,", pendingkey)
		if err := lv.l.DeletePending(&pendingkey, cache); err != nil {
			return err
		}
		return nil
	}
}

func (lv *LedgerVerifier) rollBackContractData(block *types.StateBlock, cache *ledger.Cache) error {
	extra := block.GetExtra()
	if !extra.IsZero() {
		lv.logger.Warnf("rollback contract data, block:%s, extra:%s", block.GetHash().String(), extra.String())
		t := trie.NewTrie(lv.l.DBStore(), &extra, trie.NewSimpleTrieNodePool())
		iterator := t.NewIterator(nil)
		vmContext := vmstore.NewVMContext(lv.l)
		for {
			if key, value, ok := iterator.Next(); !ok {
				break
			} else {
				if contractData, err := vmContext.GetStorageByKey(key); err == nil {
					if !bytes.Equal(contractData, value) {
						return fmt.Errorf("contract data is invalid, act: %v, exp: %v", contractData, value)
					}
					// TODO: move contract data to a new table
					lv.logger.Warnf("rollback contract data, remove storage key: %v", key)
					if err := vmContext.RemoveStorageByKey(key, cache); err == nil {
						if err := t.Remove(cache); err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					return err
				}
			}
		}
		if types.IsRewardContractAddress(types.Address(block.GetLink())) {
			preHash := block.GetPrevious()
			for {
				if preHash.IsZero() {
					break
				}
				preBlock, err := lv.l.GetStateBlockConfirmed(preHash)
				if err != nil {
					return fmt.Errorf("contract block previous not found (%s)", block.GetHash())
				}
				if preBlock.GetType() == block.GetType() && preBlock.GetLink() == block.GetLink() {
					ex := preBlock.GetExtra()
					tr := trie.NewTrie(lv.l.DBStore(), &ex, trie.NewSimpleTrieNodePool())
					iter := tr.NewIterator(nil)
					for {
						if key, value, ok := iter.Next(); !ok {
							break
						} else {
							if err := cache.Put(key, value); err != nil {
								lv.logger.Errorf("set storage error: %s", err)
							}
						}
					}
					break
				}
				preHash = preBlock.GetPrevious()
			}
		}
	}
	return nil
}

func (lv *LedgerVerifier) RollbackUnchecked(hash types.Hash) {
	// gap source
	blkLink, _, _ := lv.l.GetUncheckedBlock(hash, types.UncheckedKindLink)
	// gap previous
	blkPrevious, _, _ := lv.l.GetUncheckedBlock(hash, types.UncheckedKindPrevious)
	// gap token
	var blkToken *types.StateBlock
	var tokenId types.Hash
	if blk, err := lv.l.GetStateBlock(hash); err == nil {
		if blk.GetType() == types.ContractReward {
			input, err := lv.l.GetStateBlock(blk.GetLink())
			if err != nil {
				lv.logger.Errorf("dequeue get block link error [%s]", hash)
				return
			}
			address := types.Address(input.GetLink())
			if address == types.MintageAddress {
				var param = new(cabi.ParamMintage)
				tokenId = param.TokenId
				if err := cabi.MintageABI.UnpackMethod(param, cabi.MethodNameMintage, input.GetData()); err == nil {
					blkToken, _, _ = lv.l.GetUncheckedBlock(tokenId, types.UncheckedKindTokenInfo)
				}
			}
		}
	}

	if blkLink == nil && blkPrevious == nil && blkToken == nil {
		return
	}
	if blkLink != nil {
		err := lv.l.DeleteUncheckedBlock(hash, types.UncheckedKindLink)
		if err != nil {
			lv.logger.Errorf("Get err [%s] for hash: [%s] when delete UncheckedKindLink", err, blkLink.GetHash())
		}
		lv.RollbackUnchecked(blkLink.GetHash())
	}
	if blkPrevious != nil {
		err := lv.l.DeleteUncheckedBlock(hash, types.UncheckedKindPrevious)
		if err != nil {
			lv.logger.Errorf("Get err [%s] for hash: [%s] when delete UncheckedKindPrevious", err, blkPrevious.GetHash())
		}
		lv.RollbackUnchecked(blkPrevious.GetHash())
	}
	if blkToken != nil {
		err := lv.l.DeleteUncheckedBlock(tokenId, types.UncheckedKindTokenInfo)
		if err != nil {
			lv.logger.Errorf("Get err [%s] for hash: [%s] when delete UncheckedKindTokenInfo", err, blkToken.GetHash())
		}
		lv.RollbackUnchecked(blkToken.GetHash())
	}
}
