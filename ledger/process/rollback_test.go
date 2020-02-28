package process

import (
	"fmt"
	"testing"
	"time"

	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/storage"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/mock"
	"github.com/qlcchain/go-qlc/trie"
)

func TestProcess_Rollback(t *testing.T) {
	teardownTestCase, l, lv := setupTestCase(t)
	defer teardownTestCase(t)
	var bc, _ = mock.BlockChain(false)
	if err := lv.BlockProcess(bc[0]); err != nil {
		t.Fatal(err)
	}
	t.Log("bc hash", bc[0].GetHash())
	for i, b := range bc[1:] {
		fmt.Println(i + 1)
		fmt.Println("bc.previous", b.GetPrevious())
		if p, err := lv.Process(b); err != nil || p != Progress {
			t.Fatal(p, err)
		}
	}

	rb := bc[4]
	fmt.Println("rollback")
	fmt.Println("rollback hash: ", rb.GetHash(), rb.GetType(), rb.GetPrevious().String())
	if err := lv.Rollback(rb.GetHash()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	checkInfo(t, l)
}

func checkInfo(t *testing.T, l *ledger.Ledger) {
	addrs := make(map[types.Address]int)
	fmt.Println("----blocks----")
	err := l.GetStateBlocksConfirmed(func(block *types.StateBlock) error {
		fmt.Println(block)
		if block.GetHash() != common.GenesisBlockHash() {
			if _, ok := addrs[block.GetAddress()]; !ok {
				addrs[block.GetAddress()] = 0
			}
			return nil
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(addrs)
	fmt.Println("----frontiers----")
	fs, _ := l.GetFrontiers()
	for _, f := range fs {
		fmt.Println(f)
	}

	fmt.Println("----account----")
	for k := range addrs {
		ac, err := l.GetAccountMeta(k)
		if err != nil {
			t.Fatal(err, k)
		}
		fmt.Println("   account", ac.Address)
		for _, token := range ac.Tokens {
			fmt.Println("      token, ", token)
		}
	}

	fmt.Println("----representation----")
	for k := range addrs {
		b, err := l.GetRepresentation(k)
		if err != nil {
			if err == ledger.ErrRepresentationNotFound {
			}
		} else {
			fmt.Println(k, b)
		}
	}

	fmt.Println("----pending----")
	err = l.GetPendings(func(pendingKey *types.PendingKey, pendingInfo *types.PendingInfo) error {
		fmt.Println("      key:", pendingKey)
		fmt.Println("      info:", pendingInfo)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLedgerVerifier_BlockCacheCheck(t *testing.T) {
	teardownTestCase, _, lv := setupTestCase(t)
	defer teardownTestCase(t)
	addr := mock.Address()
	ac := mock.AccountMeta(addr)
	token := ac.Tokens[0].Type
	block := mock.StateBlock()
	block.Address = addr
	block.Token = token

	if err := lv.l.AddBlockCache(block); err != nil {
		t.Fatal(err)
	}
	if err := lv.l.AddAccountMetaCache(ac); err != nil {
		t.Fatal(err)
	}
	if err := lv.Rollback(block.GetHash()); err != nil {
		t.Fatal(err)
	}

	if b, _ := lv.l.HasBlockCache(block.GetHash()); b {
		t.Fatal()
	}
	ac, err := lv.l.GetAccountMeteCache(addr)
	if err != nil {
		t.Fatal(err)
	}
	if tm := ac.Token(token); tm != nil {
		t.Fatal(err)
	}
}

func TestLedger_Rollback_ContractData(t *testing.T) {
	teardownTestCase, l, lv := setupTestCase(t)
	defer teardownTestCase(t)

	bs := mock.ContractBlocks()
	if err := lv.BlockProcess(bs[0]); err != nil {
		t.Fatal(err)
	}
	for i, b := range bs[1:] {
		fmt.Println("index，", i+1)
		fmt.Println("bc.previous", b.GetPrevious())
		if p, err := lv.Process(b); err != nil || p != Progress {
			t.Fatal(p, err)
		}
	}
	time.Sleep(2 * time.Second)

	nodeCount := nodesCount(lv.l.DBStore(), bs[2].GetExtra())
	if nodeCount == 0 {
		t.Fatal("failed to add nodes", nodeCount)
	}

	time.Sleep(1 * time.Second)
	c, err := l.CountStateBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if int(c) != len(bs) {
		t.Fatal("block count error")
	}
	fmt.Println("block count === ", c)

	nodeCount = nodesCount(lv.l.DBStore(), bs[2].GetExtra())
	if nodeCount == 0 {
		t.Fatal("failed to add nodes", nodeCount)
	}
	fmt.Println(" node count ", nodeCount)

	if err := lv.Rollback(bs[2].GetHash()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	nodeCount = nodesCount(lv.l.DBStore(), bs[2].GetExtra())
	t.Log(nodeCount)
	if nodeCount > 0 {
		t.Fatal("failed to remove nodes", nodeCount)
	}

	fmt.Println("rollback again")
	if p, err := lv.Process(bs[2]); err != nil || p != Progress {
		t.Fatal(p, err)
	}
	time.Sleep(2 * time.Second)
	nodeCount = nodesCount(lv.l.DBStore(), bs[2].GetExtra())
	t.Log(nodeCount)
	if nodeCount == 0 {
		t.Fatal("failed to add nodes", nodeCount)
	}

	if err := lv.Rollback(bs[2].GetHash()); err != nil {
		t.Fatal(err)
	}
	fmt.Println("process again")

	time.Sleep(2 * time.Second)
	if p, err := lv.Process(bs[2]); err != nil || p != Progress {
		t.Fatal(p, err)
	}
	//time.Sleep(4 * time.Second)
	//nodeCount = nodesCount(lv.l.DBStore(), bs[2].GetExtra())
	//t.Log(nodeCount)
	//if nodeCount == 0 {
	//	t.Fatal("failed to add nodes", nodeCount)
	//}
}

func nodesCount(db storage.Store, rootHash types.Hash) int {
	tr := trie.NewTrie(db, &rootHash, trie.NewSimpleTrieNodePool())
	iterator := tr.NewIterator(nil)
	counter := 0
	for {
		if _, _, ok := iterator.Next(); !ok {
			break
		} else {
			counter++
		}
	}
	return counter
}
