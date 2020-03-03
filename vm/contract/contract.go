/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package contract

import (
	"errors"

	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/statedb"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/log"
	"github.com/qlcchain/go-qlc/vm/abi"
	cabi "github.com/qlcchain/go-qlc/vm/contract/abi"
	"github.com/qlcchain/go-qlc/vm/vmstore"
)

const (
	SpecVerInvalid = iota
	SpecVer1       = 1
	SpecVer2       = 2
)

var (
	logger = log.NewLogger("contract")

	ErrToken            = errors.New("token err")
	ErrUnpackMethod     = errors.New("unpack method err")
	ErrPackMethod       = errors.New("pack method err")
	ErrNotEnoughPledge  = errors.New("not enough pledge")
	ErrCheckParam       = errors.New("check param err")
	ErrSetStorage       = errors.New("set storage err")
	ErrCalcAmount       = errors.New("calc amount err")
	ErrNotEnoughFee     = errors.New("not enough fee")
	ErrGetVerifier      = errors.New("get verifier err")
	ErrAccountInvalid   = errors.New("invalid account")
	ErrAccountNotExist  = errors.New("account not exist")
	ErrGetNodeHeight    = errors.New("get node height err")
	ErrEndHeightInvalid = errors.New("invalid claim end height")
	ErrClaimRepeat      = errors.New("claim reward repeatedly")
	ErrGetRewardHistory = errors.New("get reward history err")
	ErrVerifierNum      = errors.New("verifier num err")
	ErrPledgeNotReady   = errors.New("pledge is not ready")
)

//ContractBlock generated by contract
type ContractBlock struct {
	VMContext *vmstore.VMContext
	Block     *types.StateBlock
	ToAddress types.Address
	BlockType types.BlockType
	Amount    types.Balance
	Token     types.Hash
	Data      []byte
}

type Describe struct {
	specVer       int
	withSignature bool
	withPending   bool
	withPovState  bool
	withWork      bool
}

func (d Describe) GetVersion() int {
	return d.specVer
}
func (d Describe) WithSignature() bool {
	return d.withSignature
}
func (d Describe) WithPending() bool {
	return d.withPending
}
func (d Describe) WithPovState() bool {
	return d.withPovState
}
func (d Describe) WithWork() bool {
	return d.withWork
}

type Contract interface {
	// Contract meta describe
	GetDescribe() Describe
	// Target receiver address
	GetTargetReceiver(ctx *vmstore.VMContext, block *types.StateBlock) (types.Address, error)

	GetFee(ctx *vmstore.VMContext, block *types.StateBlock) (types.Balance, error)
	// check status, update state
	DoReceive(ctx *vmstore.VMContext, block *types.StateBlock, input *types.StateBlock) ([]*ContractBlock, error)
	// refund data at receive error
	GetRefundData() []byte

	// DoPending generate pending info from send block
	DoPending(block *types.StateBlock) (*types.PendingKey, *types.PendingInfo, error)
	// ProcessSend verify or update StateBlock.Data
	DoSend(ctx *vmstore.VMContext, block *types.StateBlock) error

	// ProcessSend verify or update StateBlock.Data
	ProcessSend(ctx *vmstore.VMContext, block *types.StateBlock) (*types.PendingKey, *types.PendingInfo, error)
	DoGap(ctx *vmstore.VMContext, block *types.StateBlock) (common.ContractGapType, interface{}, error)

	DoSendOnPov(ctx *vmstore.VMContext, csdb *statedb.PovContractStateDB, povHeight uint64, block *types.StateBlock) error
	DoReceiveOnPov(ctx *vmstore.VMContext, csdb *statedb.PovContractStateDB, povHeight uint64, block *types.StateBlock, input *types.StateBlock) error
}

type BaseContract struct {
	Describe
}

func (c *BaseContract) GetDescribe() Describe {
	return c.Describe
}

func (c *BaseContract) GetTargetReceiver(ctx *vmstore.VMContext, block *types.StateBlock) (types.Address, error) {
	return types.ZeroAddress, nil
}

func (c *BaseContract) GetFee(ctx *vmstore.VMContext, block *types.StateBlock) (types.Balance, error) {
	return types.ZeroBalance, nil
}

// refund data at receive error
func (c *BaseContract) GetRefundData() []byte {
	return []byte{1}
}

// DoPending generate pending info from send block
func (c *BaseContract) DoPending(block *types.StateBlock) (*types.PendingKey, *types.PendingInfo, error) {
	return nil, nil, nil
}

// ProcessSend verify or update StateBlock.Data
func (c *BaseContract) DoSend(ctx *vmstore.VMContext, block *types.StateBlock) error {
	return errors.New("not implemented")
}

// check status, update state
func (c *BaseContract) DoReceive(ctx *vmstore.VMContext, block *types.StateBlock, input *types.StateBlock) ([]*ContractBlock, error) {
	return nil, errors.New("not implemented")
}

// ProcessSend verify or update StateBlock.Data
func (c *BaseContract) ProcessSend(ctx *vmstore.VMContext, block *types.StateBlock) (*types.PendingKey, *types.PendingInfo, error) {
	return nil, nil, errors.New("not implemented")
}

func (c *BaseContract) DoGap(ctx *vmstore.VMContext, block *types.StateBlock) (common.ContractGapType, interface{}, error) {
	return common.ContractNoGap, nil, nil
}

func (c *BaseContract) DoSendOnPov(ctx *vmstore.VMContext, csdb *statedb.PovContractStateDB, povHeight uint64, block *types.StateBlock) error {
	return errors.New("not implemented")
}

func (c *BaseContract) DoReceiveOnPov(ctx *vmstore.VMContext, csdb *statedb.PovContractStateDB, povHeight uint64, block *types.StateBlock, input *types.StateBlock) error {
	return errors.New("not implemented")
}

type qlcChainContract struct {
	m       map[string]Contract
	abi     abi.ABIContract
	abiJson string
}

var qlcAllChainContracts = map[types.Address]*qlcChainContract{
	types.MintageAddress: {
		map[string]Contract{
			cabi.MethodNameMintage: &Mintage{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer1,
						withSignature: true,
						withWork:      true,
					},
				},
			},
			cabi.MethodNameMintageWithdraw: &WithdrawMintage{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer1,
						withSignature: true,
						withWork:      true,
					},
				},
			},
		},
		cabi.MintageABI,
		cabi.JsonMintage,
	},
	types.NEP5PledgeAddress: {
		map[string]Contract{
			cabi.MethodNEP5Pledge: &Nep5Pledge{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer1,
						withSignature: true,
						withWork:      true,
					},
				},
			},
			cabi.MethodWithdrawNEP5Pledge: &WithdrawNep5Pledge{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer1,
						withSignature: true,
						withWork:      true,
					},
				},
			},
		},
		cabi.NEP5PledgeABI,
		cabi.JsonNEP5Pledge,
	},
	types.RewardsAddress: {
		map[string]Contract{
			cabi.MethodNameAirdropRewards: &AirdropRewords{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:     SpecVer1,
						withPending: true,
					},
				},
			},
			cabi.MethodNameConfidantRewards: &ConfidantRewards{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:     SpecVer1,
						withPending: true,
					},
				},
			},
		},
		cabi.RewardsABI,
		cabi.JsonRewards,
	},
	types.BlackHoleAddress: {
		m: map[string]Contract{
			cabi.MethodNameDestroy: &BlackHole{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:     SpecVer2,
						withPending: true,
					},
				},
			},
		},
		abi:     cabi.BlackHoleABI,
		abiJson: cabi.JsonDestroy,
	},
	types.MinerAddress: {
		map[string]Contract{
			cabi.MethodNameMinerReward: &MinerReward{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPending:   true,
						withWork:      true,
					},
				},
			},
		},
		cabi.MinerABI,
		cabi.JsonMiner,
	},
	types.RepAddress: {
		map[string]Contract{
			cabi.MethodNameRepReward: &RepReward{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPending:   true,
						withWork:      true,
					},
				},
			},
		},
		cabi.RepABI,
		cabi.JsonRep,
	},
	types.SettlementAddress: {
		map[string]Contract{
			cabi.MethodNameCreateContract:    &CreateContract{},
			cabi.MethodNameSignContract:      &SignContract{},
			cabi.MethodNameProcessCDR:        &ProcessCDR{},
			cabi.MethodNameAddPreStop:        &AddPreStop{},
			cabi.MethodNameUpdatePreStop:     &UpdatePreStop{},
			cabi.MethodNameRemovePreStop:     &RemovePreStop{},
			cabi.MethodNameAddNextStop:       &AddNextStop{},
			cabi.MethodNameUpdateNextStop:    &UpdateNextStop{},
			cabi.MethodNameRemoveNextStop:    &RemoveNextStop{},
			cabi.MethodNameTerminateContract: &TerminateContract{},
		},
		cabi.SettlementABI,
		cabi.JsonSettlement,
	},
	types.PubKeyDistributionAddress: {
		map[string]Contract{
			cabi.MethodNamePKDVerifierRegister: &VerifierRegister{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withWork:      true,
					},
				},
			},
			cabi.MethodNamePKDVerifierUnregister: &VerifierUnregister{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withWork:      true,
					},
				},
			},
			cabi.MethodNamePKDPublish: &Publish{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPovState:  true,
					},
				},
			},
			cabi.MethodNamePKDUnPublish: &UnPublish{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
					},
				},
			},
			cabi.MethodNamePKDOracle: &Oracle{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPovState:  true,
					},
				},
			},
			cabi.MethodNamePKDReward: &PKDReward{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPending:   true,
						withWork:      true,
					},
				},
			},
			cabi.MethodNamePKDVerifierHeart: &VerifierHeart{
				BaseContract: BaseContract{
					Describe: Describe{
						specVer:       SpecVer2,
						withSignature: true,
						withPovState:  true,
					},
				},
			},
		},
		cabi.PublicKeyDistributionABI,
		cabi.JsonPublicKeyDistribution,
	},
}

func GetChainContract(addr types.Address, methodSelector []byte) (Contract, bool, error) {
	if p, ok := qlcAllChainContracts[addr]; ok {
		if method, err := p.abi.MethodById(methodSelector); err == nil {
			c, ok := p.m[method.Name]
			return c, ok, nil
		} else {
			return nil, ok, errors.New("abi: method not found")
		}
	}
	return nil, false, nil
}

func GetChainContractName(addr types.Address, methodSelector []byte) (string, bool, error) {
	if p, ok := qlcAllChainContracts[addr]; ok {
		if method, err := p.abi.MethodById(methodSelector); err == nil {
			_, ok := p.m[method.Name]
			return method.Name, ok, nil
		} else {
			return "", ok, errors.New("abi: method not found")
		}
	}

	return "", false, nil
}

func IsChainContract(addr types.Address) bool {
	if _, ok := qlcAllChainContracts[addr]; ok {
		return true
	}
	return false
}

func GetAbiByContractAddress(addr types.Address) (string, error) {
	if contract, ok := qlcAllChainContracts[addr]; ok {
		return contract.abiJson, nil
	}
	return "", errors.New("contract not found")
}

func SetMinMintageTimeForTest() {
	minMintageTime = &timeSpan{seconds: 1}
}
