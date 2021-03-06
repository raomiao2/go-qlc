package abi

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
	"github.com/qlcchain/go-qlc/common/vmcontract/contractaddress"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/vm/abi"
	"github.com/qlcchain/go-qlc/vm/vmstore"
)

const (
	JsonPublicKeyDistribution = `
	[
		{"type":"function","name":"PKDVerifierRegister","inputs":[
			{"name":"vType","type":"uint32"},
			{"name":"vInfo","type":"string"},
			{"name":"vKey","type":"uint8[]"}
		]},
		{"type":"function","name":"PKDVerifierUnregister","inputs":[
			{"name":"vType","type":"uint32"}
		]},
		{"type":"function","name":"PKDVerifierHeart","inputs":[
			{"name":"vType","type":"uint32[]"}
		]},
		{"type":"function","name":"PKDOracle","inputs":[
			{"name":"oType","type":"uint32"},
			{"name":"oID","type":"hash"},
			{"name":"keyType","type":"uint16"},
			{"name":"pubKey","type":"uint8[]"},
			{"name":"code","type":"string"},
			{"name":"hash","type":"hash"}
		]},
		{"type":"function","name":"PKDPublish","inputs":[
			{"name":"pType","type":"uint32"},
			{"name":"pID","type":"hash"},
			{"name":"keyType","type":"uint16"},
			{"name":"pubKey","type":"uint8[]"},
			{"name":"verifiers","type":"address[]"},
			{"name":"codes","type":"hash[]"},
			{"name":"fee","type":"uint256"}
		]},
		{"type":"function","name":"PKDUnPublish","inputs":[
			{"name":"pType","type":"uint32"},
			{"name":"pID","type":"hash"},
			{"name":"keyType","type":"uint16"},
			{"name":"pubKey","type":"uint8[]"},
			{"name":"hash","type":"hash"}
		]},
		{"type":"function","name":"PKDReward","inputs":[
			{"name":"account","type":"address"},
			{"name":"beneficial","type":"address"},
			{"name":"endHeight","type":"uint64"},
			{"name":"rewardAmount","type":"uint256"}
		]},
		{"type":"variable","name":"PKDVerifierInfo","inputs":[
			{"name":"vInfo","type":"string"},
			{"name":"vKey","type":"uint8[]"},
			{"name":"valid","type":"bool"}
		]},
		{"type":"variable","name":"PKDOracleInfo","inputs":[
			{"name":"code","type":"string"},
			{"name":"keyType","type":"uint16"},
			{"name":"pubKey","type":"uint8[]"}
		]},
		{"type":"variable","name":"PKDPublishInfo","inputs":[
			{"name":"account","type":"address"},
			{"name":"verifiers","type":"address[]"},
			{"name":"codes","type":"hash[]"},
			{"name":"fee","type":"uint256"},
			{"name":"valid","type":"bool"},
			{"name":"keyType","type":"uint16"},
			{"name":"pubKey","type":"uint8[]"}
		]},
		{"type":"variable","name":"PKDRewardInfo","inputs":[
			{"name":"beneficial","type":"address"},
			{"name":"endHeight","type":"uint64"},
			{"name":"rewardAmount","type":"uint256"},
			{"name":"timestamp","type":"int64"}
		]}
	]`

	MethodNamePKDVerifierRegister   = "PKDVerifierRegister"
	MethodNamePKDVerifierUnregister = "PKDVerifierUnregister"
	MethodNamePKDVerifierHeart      = "PKDVerifierHeart"
	MethodNamePKDOracle             = "PKDOracle"
	MethodNamePKDPublish            = "PKDPublish"
	MethodNamePKDUnPublish          = "PKDUnPublish"
	MethodNamePKDReward             = "PKDReward"
	VariableNamePKDPublishInfo      = "PKDPublishInfo"
	VariableNamePKDOracleInfo       = "PKDOracleInfo"
	VariableNamePKDVerifierInfo     = "PKDVerifierInfo"
	VariableNamePKDRewardInfo       = "PKDRewardInfo"
)

var (
	PublicKeyDistributionABI, _ = abi.JSONToABIContract(strings.NewReader(JsonPublicKeyDistribution))
)

const (
	PKDStorageTypeVerifier byte = iota
	PKDStorageTypePublisher
	PKDStorageTypeOracle
	PKDStorageTypeReward
)

const (
	// contractAddr(32) + pkdType(1) + type(4) + account(32) => info + valid
	VerifierTypeIndexS = 1 + types.AddressSize
	VerifierTypeIndexE = VerifierTypeIndexS + 4
	VerifierAccIndexS  = VerifierTypeIndexE
	VerifierAccIndexE  = VerifierAccIndexS + 32
)

type VerifierRegInfo struct {
	Account types.Address
	VType   uint32
	VInfo   string
	VKey    []byte
}

type VerifierUnRegInfo struct {
	Account types.Address
	VType   uint32
}

type VerifierStorage struct {
	VInfo string
	VKey  []byte
	Valid bool
}

type VerifierHeartInfo struct {
	VType []uint32
}

func VerifierRegInfoCheck(ctx *vmstore.VMContext, account types.Address, vType uint32, vInfo string, vKey []byte) error {
	switch vType {
	case common.OracleTypeEmail:
		if !util.VerifyEmailFormat(vInfo) {
			return fmt.Errorf("invalid email format (%s)", vInfo)
		}

		if CheckVerifierInfoExist(ctx, account, vType, vInfo, vKey) {
			return fmt.Errorf("email has been registered (%s)", vInfo)
		}
	case common.OracleTypeWeChat:
		if CheckVerifierInfoExist(ctx, account, vType, vInfo, vKey) {
			return fmt.Errorf("weChat id has been registered (%s)", vInfo)
		}
	default:
		return fmt.Errorf("invalid verifier type(%s)", common.OracleTypeToString(vType))
	}

	return nil
}

func VerifierUnRegInfoCheck(ctx *vmstore.VMContext, account types.Address, vType uint32) error {
	switch vType {
	case common.OracleTypeEmail, common.OracleTypeWeChat:
		if !CheckVerifierExist(ctx, account, vType) {
			return fmt.Errorf("there is no valid registered info")
		}
	default:
		return fmt.Errorf("invalid verifier type(%s)", common.OracleTypeToString(vType))
	}
	return nil
}

func VerifierPledgeCheck(ctx *vmstore.VMContext, account types.Address) error {
	am, err := ctx.GetAccountMeta(account)
	if err != nil {
		return err
	}

	if am.CoinOracle.Compare(common.MinVerifierPledgeAmount) == types.BalanceCompSmaller {
		return fmt.Errorf("%s have not enough oracle pledge %s", account, am.CoinOracle)
	}

	return nil
}

func CheckVerifierInfoExist(ctx *vmstore.VMContext, account types.Address, vType uint32, vInfo string, vKey []byte) bool {
	var key []byte
	key = append(key, PKDStorageTypeVerifier)
	key = append(key, util.BE_Uint32ToBytes(vType)...)
	key = append(key, account[:]...)
	val, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return false
	}

	var info VerifierStorage
	err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDVerifierInfo, val)
	if err != nil {
		return false
	}

	if info.Valid && info.VInfo == vInfo && bytes.Equal(info.VKey, vKey) {
		return true
	}

	return false
}

func CheckVerifierExist(ctx *vmstore.VMContext, account types.Address, vType uint32) bool {
	var key []byte
	key = append(key, PKDStorageTypeVerifier)
	key = append(key, util.BE_Uint32ToBytes(vType)...)
	key = append(key, account[:]...)
	val, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return false
	}

	var info VerifierStorage
	err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDVerifierInfo, val)
	if err != nil {
		return false
	}

	return info.Valid
}

func GetVerifierInfoByAccountAndType(ctx *vmstore.VMContext, account types.Address, vType uint32) (*VerifierStorage, error) {
	var key []byte
	key = append(key, PKDStorageTypeVerifier)
	key = append(key, util.BE_Uint32ToBytes(vType)...)
	key = append(key, account[:]...)
	val, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return nil, err
	}

	vs := new(VerifierStorage)
	err = PublicKeyDistributionABI.UnpackVariable(vs, VariableNamePKDVerifierInfo, val)
	if err != nil || !vs.Valid {
		return nil, err
	}

	return vs, nil
}

func GetAllVerifiers(store ledger.Store) ([]*VerifierRegInfo, error) {
	vrs := make([]*VerifierRegInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeVerifier)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)

	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		var vs VerifierStorage
		err := PublicKeyDistributionABI.UnpackVariable(&vs, VariableNamePKDVerifierInfo, value)
		if err != nil || !vs.Valid {
			return err
		}

		addr, err := types.BytesToAddress(key[VerifierAccIndexS:VerifierAccIndexE])
		if err != nil {
			return err
		}

		vr := &VerifierRegInfo{
			Account: addr,
			VType:   util.BE_BytesToUint32(key[VerifierTypeIndexS:VerifierTypeIndexE]),
			VInfo:   vs.VInfo,
			VKey:    vs.VKey,
		}

		vrs = append(vrs, vr)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return vrs, nil
}

func GetVerifiersByType(store ledger.Store, vType uint32) ([]*VerifierRegInfo, error) {
	vrs := make([]*VerifierRegInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeVerifier)
	itKey = append(itKey, util.BE_Uint32ToBytes(vType)...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		var vs VerifierStorage
		err := PublicKeyDistributionABI.UnpackVariable(&vs, VariableNamePKDVerifierInfo, value)
		if err != nil || !vs.Valid {
			return err
		}

		addr, err := types.BytesToAddress(key[VerifierAccIndexS:VerifierAccIndexE])
		if err != nil {
			return err
		}

		vr := &VerifierRegInfo{
			Account: addr,
			VType:   util.BE_BytesToUint32(key[VerifierTypeIndexS:VerifierTypeIndexE]),
			VInfo:   vs.VInfo,
			VKey:    vs.VKey,
		}

		vrs = append(vrs, vr)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return vrs, nil
}

func GetVerifiersByAccount(store ledger.Store, account types.Address) ([]*VerifierRegInfo, error) {
	vrs := make([]*VerifierRegInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeVerifier)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		if !bytes.Equal(key[VerifierAccIndexS:VerifierAccIndexE], account[:]) {
			return nil
		}

		var vs VerifierStorage
		err := PublicKeyDistributionABI.UnpackVariable(&vs, VariableNamePKDVerifierInfo, value)
		if err != nil || !vs.Valid {
			return err
		}

		vr := &VerifierRegInfo{
			Account: account,
			VType:   util.BE_BytesToUint32(key[VerifierTypeIndexS:VerifierTypeIndexE]),
			VInfo:   vs.VInfo,
			VKey:    vs.VKey,
		}

		vrs = append(vrs, vr)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return vrs, nil
}

const (
	// contractAddr(32) + pkdType(1) + type(4) + id(32) + pk(32) + blockHash(32) + account(32) => code(16)
	OracleTypeIndexS = 1 + types.AddressSize
	OracleTypeIndexE = OracleTypeIndexS + 4
	OracleIdIndexS   = OracleTypeIndexE
	OracleIdIndexE   = OracleIdIndexS + sha256.Size
	OraclePkIndexS   = OracleIdIndexE
	OraclePkIndexE   = OraclePkIndexS + ed25519.PublicKeySize
	OracleHashIndexS = OraclePkIndexE
	OracleHashIndexE = OracleHashIndexS + 32
	OracleAccIndexS  = OracleHashIndexE
	OracleAccIndexE  = OracleAccIndexS + 32
)

type OracleInfo struct {
	Account types.Address
	OType   uint32
	OID     types.Hash
	KeyType uint16
	PubKey  []byte
	Code    string
	Hash    types.Hash
}

type CodeInfo struct {
	Code    string
	KeyType uint16
	PubKey  []byte
}

func OracleInfoCheck(ctx *vmstore.VMContext, account types.Address, ot uint32, id types.Hash, kt uint16, pk []byte, code string, hash types.Hash) error {
	switch ot {
	case common.OracleTypeEmail, common.OracleTypeWeChat:
		switch kt {
		case common.PublicKeyTypeED25519:
			if len(pk) != ed25519.PublicKeySize {
				return fmt.Errorf("pk len err")
			}
		case common.PublicKeyTypeRSA4096:
			if len(pk) != 512 {
				return fmt.Errorf("pk len err")
			}
		default:
			return fmt.Errorf("key type err")
		}

		info := GetPublishInfo(ctx, ot, id, kt, pk, hash)
		if info == nil {
			return fmt.Errorf("invalid oracle")
		}

		index := -1
		for i, v := range info.Verifiers {
			if v == account {
				index = i
				break
			}
		}

		if index == -1 {
			return fmt.Errorf("get verifier err")
		}

		codeComb := append([]byte(types.NewHexBytesFromData(pk).String()), []byte(code)...)
		codeHash, err := types.Sha256HashData(codeComb)
		if err != nil {
			return err
		}

		if codeHash != info.Codes[index] {
			return fmt.Errorf("verification code err")
		}
	default:
		return fmt.Errorf("puslish type(%s) err", common.OracleTypeToString(ot))
	}

	return nil
}

func GetAllOracleInfo(store ledger.Store) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		ot := util.BE_BytesToUint32(key[OracleTypeIndexS:OracleTypeIndexE])

		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		addr, err := types.BytesToAddress(key[OracleAccIndexS:OracleAccIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[OracleIdIndexS:OracleIdIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: addr,
			OType:   ot,
			OID:     id,
			KeyType: info.KeyType,
			PubKey:  info.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByType(store ledger.Store, ot uint32) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	itKey = append(itKey, util.BE_Uint32ToBytes(ot)...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		addr, err := types.BytesToAddress(key[OracleAccIndexS:OracleAccIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[OracleIdIndexS:OracleIdIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: addr,
			OType:   ot,
			OID:     id,
			KeyType: info.KeyType,
			PubKey:  info.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByTypeAndID(store ledger.Store, ot uint32, id types.Hash) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	itKey = append(itKey, util.BE_Uint32ToBytes(ot)...)
	itKey = append(itKey, id[:]...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		addr, err := types.BytesToAddress(key[OracleAccIndexS:OracleAccIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: addr,
			OType:   ot,
			OID:     id,
			KeyType: info.KeyType,
			PubKey:  info.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByTypeAndIDAndPk(store ledger.Store, ot uint32, id types.Hash, kt uint16, pk []byte) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	kh := common.PublicKeyWithTypeHash(kt, pk)
	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	itKey = append(itKey, util.BE_Uint32ToBytes(ot)...)
	itKey = append(itKey, id[:]...)
	itKey = append(itKey, kh...)

	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		addr, err := types.BytesToAddress(key[OracleAccIndexS:OracleAccIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: addr,
			OType:   ot,
			OID:     id,
			KeyType: kt,
			PubKey:  pk,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByAccount(store ledger.Store, account types.Address) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		if !bytes.Equal(account[:], key[OracleAccIndexS:OracleAccIndexE]) {
			return nil
		}

		ot := util.BE_BytesToUint32(key[OracleTypeIndexS:OracleTypeIndexE])

		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[OracleIdIndexS:OracleIdIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: account,
			OType:   ot,
			OID:     id,
			KeyType: info.KeyType,
			PubKey:  info.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByAccountAndType(store ledger.Store, account types.Address, ot uint32) []*OracleInfo {
	ois := make([]*OracleInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	itKey = append(itKey, util.BE_Uint32ToBytes(ot)...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		if !bytes.Equal(account[:], key[OracleAccIndexS:OracleAccIndexE]) {
			return nil
		}

		hash, err := types.BytesToHash(key[OracleHashIndexS:OracleHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[OracleIdIndexS:OracleIdIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: account,
			OType:   ot,
			OID:     id,
			KeyType: info.KeyType,
			PubKey:  info.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func GetOracleInfoByHash(store ledger.Store, hash types.Hash) []*OracleInfo {
	ois := make([]*OracleInfo, 0)
	childHash, err := store.GetBlockChild(hash)
	if err != nil {
		return nil
	}

	block, err := store.GetStateBlockConfirmed(childHash)
	if err != nil {
		return nil
	}

	var pi PublishInfo
	err = PublicKeyDistributionABI.UnpackMethod(&pi, MethodNamePKDPublish, block.GetData())
	if err != nil {
		return nil
	}

	kh := common.PublicKeyWithTypeHash(pi.KeyType, pi.PubKey)
	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypeOracle)
	itKey = append(itKey, util.BE_Uint32ToBytes(pi.PType)...)
	itKey = append(itKey, pi.PID[:]...)
	itKey = append(itKey, kh...)
	itKey = append(itKey, hash[:]...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err = iterator.Next(itKey, func(key []byte, value []byte) error {
		addr, err := types.BytesToAddress(key[OracleAccIndexS:OracleAccIndexE])
		if err != nil {
			return err
		}

		var info CodeInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDOracleInfo, value)
		if err != nil {
			return err
		}

		oi := &OracleInfo{
			Account: addr,
			OType:   pi.PType,
			OID:     pi.PID,
			KeyType: pi.KeyType,
			PubKey:  pi.PubKey,
			Code:    info.Code,
			Hash:    hash,
		}

		ois = append(ois, oi)
		return nil
	})
	if err != nil {
		return nil
	}

	return ois
}

func CheckOracleInfoExist(ctx *vmstore.VMContext, account types.Address, ot uint32, id types.Hash, kt uint16, pk []byte, hash types.Hash) bool {
	var key []byte
	kh := common.PublicKeyWithTypeHash(kt, pk)
	key = append(key, PKDStorageTypeOracle)
	key = append(key, util.BE_Uint32ToBytes(ot)...)
	key = append(key, id[:]...)
	key = append(key, kh...)
	key = append(key, hash[:]...)
	key = append(key, account[:]...)
	_, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return false
	}
	return true
}

const (
	// each accounts or different accounts can send type + id + pk multiple times, so we need a flag to distinguish,
	// here we use previous block's hash
	//  contractAddr(32) + pkdType(1) + type(4) + id(32) + pk(32) + prevHash(32) => account + verifiers + codes + fee
	PublishTypeIndexS = 1 + types.AddressSize
	PublishTypeIndexE = PublishTypeIndexS + 4
	PublishIdIndexS   = PublishTypeIndexE
	PublishIdIndexE   = PublishIdIndexS + sha256.Size
	PublishPkIndexS   = PublishIdIndexE
	PublishPkIndexE   = PublishPkIndexS + ed25519.PublicKeySize
	PublishHashIndexS = PublishPkIndexE
	PublishHashIndexE = PublishHashIndexS + 32
)

type PublishInfoKey struct {
	PType  uint32
	PID    types.Hash
	PubKey []byte
	Hash   types.Hash
}

func (k *PublishInfoKey) ToRawKey() []byte {
	var psRawKey []byte
	psRawKey = append(psRawKey, util.BE_Uint32ToBytes(k.PType)...)
	psRawKey = append(psRawKey, k.PID[:]...)
	psRawKey = append(psRawKey, k.PubKey...)
	psRawKey = append(psRawKey, k.Hash.Bytes()...)
	return psRawKey
}

type PublishInfo struct {
	Account   types.Address
	PType     uint32
	PID       types.Hash
	KeyType   uint16
	PubKey    []byte
	Verifiers []types.Address
	Codes     []types.Hash
	Fee       *big.Int
	Hash      types.Hash
}

type UnPublishInfo struct {
	Account types.Address
	PType   uint32
	PID     types.Hash
	KeyType uint16
	PubKey  []byte
	Hash    types.Hash
}

type PubKeyInfo struct {
	Account   types.Address
	Verifiers []types.Address
	Codes     []types.Hash
	Fee       *big.Int
	Valid     bool
	KeyType   uint16
	PubKey    []byte
}

func PublishInfoCheck(ctx *vmstore.VMContext, account types.Address, pt uint32, id types.Hash, kt uint16, pk []byte, fee types.Balance) error {
	switch pt {
	case common.OracleTypeEmail, common.OracleTypeWeChat:
		if fee.Compare(common.PublishCost) == types.BalanceCompSmaller {
			return fmt.Errorf("fee is not enough")
		}

		switch kt {
		case common.PublicKeyTypeED25519:
			if len(pk) != ed25519.PublicKeySize {
				return fmt.Errorf("pk len err")
			}
		case common.PublicKeyTypeRSA4096:
			if len(pk) != 512 {
				return fmt.Errorf("pk len err")
			}
		default:
			return fmt.Errorf("key type err")
		}
	default:
		return fmt.Errorf("puslish type(%s) err", common.OracleTypeToString(pt))
	}

	return nil
}

func UnPublishInfoCheck(ctx *vmstore.VMContext, account types.Address, pt uint32, id types.Hash, kt uint16, pk []byte, hash types.Hash) error {
	switch pt {
	case common.OracleTypeEmail, common.OracleTypeWeChat:
		switch kt {
		case common.PublicKeyTypeED25519:
			if len(pk) != ed25519.PublicKeySize {
				return fmt.Errorf("pk len err")
			}
		case common.PublicKeyTypeRSA4096:
			if len(pk) != 512 {
				return fmt.Errorf("pk len err")
			}
		default:
			return fmt.Errorf("key type err")
		}

		if !CheckPublishInfoExist(ctx, account, pt, id, kt, pk, hash) {
			return fmt.Errorf("there is no valid kind(%s) of id(%s) for(%s)", common.OracleTypeToString(pt), id, account)
		}
	default:
		return fmt.Errorf("puslish type(%s) err", common.OracleTypeToString(pt))
	}

	return nil
}

func CheckPublishInfoExist(ctx *vmstore.VMContext, account types.Address, pt uint32, id types.Hash, kt uint16, pk []byte, hash types.Hash) bool {
	var key []byte
	kh := common.PublicKeyWithTypeHash(kt, pk)
	key = append(key, PKDStorageTypePublisher)
	key = append(key, util.BE_Uint32ToBytes(pt)...)
	key = append(key, id[:]...)
	key = append(key, kh...)
	key = append(key, hash[:]...)
	data, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return false
	}

	var info PubKeyInfo
	err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, data)
	if err != nil || !info.Valid || account != info.Account {
		return false
	}

	return true
}

func GetPublishInfoByKey(ctx *vmstore.VMContext, pt uint32, pid types.Hash, kt uint16, pk []byte, blkHash types.Hash) *PubKeyInfo {
	var key []byte
	kh := common.PublicKeyWithTypeHash(kt, pk)
	key = append(key, PKDStorageTypePublisher)
	key = append(key, util.BE_Uint32ToBytes(pt)...)
	key = append(key, pid[:]...)
	key = append(key, kh...)
	key = append(key, blkHash.Bytes()...)

	data, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return nil
	}

	var info PubKeyInfo
	err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, data)
	if err != nil || !info.Valid {
		return nil
	}

	return &info
}

func GetPublishInfoByTypeAndId(store ledger.Store, pt uint32, id types.Hash) []*PublishInfo {
	pis := make([]*PublishInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypePublisher)
	itKey = append(itKey, util.BE_Uint32ToBytes(pt)...)
	itKey = append(itKey, id[:]...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		hash, err := types.BytesToHash(key[PublishHashIndexS:PublishHashIndexE])
		if err != nil {
			return err
		}

		var info PubKeyInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, value)
		if err != nil || !info.Valid {
			return err
		}

		pi := &PublishInfo{
			Account:   info.Account,
			PType:     pt,
			PID:       id,
			KeyType:   info.KeyType,
			PubKey:    info.PubKey,
			Verifiers: info.Verifiers,
			Codes:     info.Codes,
			Fee:       info.Fee,
			Hash:      hash,
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil
	}

	return pis
}

func GetAllPublishInfo(store ledger.Store) []*PublishInfo {
	pis := make([]*PublishInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypePublisher)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		pt := util.BE_BytesToUint32(key[PublishTypeIndexS:PublishTypeIndexE])

		hash, err := types.BytesToHash(key[PublishHashIndexS:PublishHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[PublishIdIndexS:PublishIdIndexE])
		if err != nil {
			return err
		}

		var info PubKeyInfo
		err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, value)
		if err != nil || !info.Valid {
			return err
		}

		pi := &PublishInfo{
			Account:   info.Account,
			PType:     pt,
			PID:       id,
			KeyType:   info.KeyType,
			PubKey:    info.PubKey,
			Verifiers: info.Verifiers,
			Codes:     info.Codes,
			Fee:       info.Fee,
			Hash:      hash,
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil
	}

	return pis
}

func GetPublishInfoByType(store ledger.Store, pt uint32) []*PublishInfo {
	pis := make([]*PublishInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypePublisher)
	itKey = append(itKey, util.BE_Uint32ToBytes(pt)...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		var info PubKeyInfo
		err := PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, value)
		if err != nil || !info.Valid {
			return err
		}

		hash, err := types.BytesToHash(key[PublishHashIndexS:PublishHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[PublishIdIndexS:PublishIdIndexE])
		if err != nil {
			return err
		}

		pi := &PublishInfo{
			Account:   info.Account,
			PType:     pt,
			PID:       id,
			KeyType:   info.KeyType,
			PubKey:    info.PubKey,
			Verifiers: info.Verifiers,
			Codes:     info.Codes,
			Fee:       info.Fee,
			Hash:      hash,
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil
	}

	return pis
}

func GetPublishInfoByAccount(store ledger.Store, account types.Address) []*PublishInfo {
	pis := make([]*PublishInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypePublisher)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		var info PubKeyInfo
		err := PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, value)
		if err != nil || !info.Valid {
			return err
		}

		if account != info.Account {
			return nil
		}

		hash, err := types.BytesToHash(key[PublishHashIndexS:PublishHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[PublishIdIndexS:PublishIdIndexE])
		if err != nil {
			return err
		}

		pi := &PublishInfo{
			Account:   account,
			PType:     util.BE_BytesToUint32(key[PublishTypeIndexS:PublishTypeIndexE]),
			PID:       id,
			KeyType:   info.KeyType,
			PubKey:    info.PubKey,
			Verifiers: info.Verifiers,
			Codes:     info.Codes,
			Fee:       info.Fee,
			Hash:      hash,
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil
	}

	return pis
}

func GetPublishInfoByAccountAndType(store ledger.Store, account types.Address, pt uint32) []*PublishInfo {
	pis := make([]*PublishInfo, 0)

	itKey := append(contractaddress.PubKeyDistributionAddress[:], PKDStorageTypePublisher)
	itKey = append(itKey, util.BE_Uint32ToBytes(pt)...)
	iterator := store.NewVMIterator(&contractaddress.PubKeyDistributionAddress)
	err := iterator.Next(itKey, func(key []byte, value []byte) error {
		var info PubKeyInfo
		err := PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, value)
		if err != nil || !info.Valid {
			return err
		}

		if account != info.Account {
			return nil
		}

		hash, err := types.BytesToHash(key[PublishHashIndexS:PublishHashIndexE])
		if err != nil {
			return err
		}

		id, err := types.BytesToHash(key[PublishIdIndexS:PublishIdIndexE])
		if err != nil {
			return err
		}

		pi := &PublishInfo{
			Account:   account,
			PType:     pt,
			PID:       id,
			KeyType:   info.KeyType,
			PubKey:    info.PubKey,
			Verifiers: info.Verifiers,
			Codes:     info.Codes,
			Fee:       info.Fee,
			Hash:      hash,
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil
	}

	return pis
}

func GetPublishInfo(ctx *vmstore.VMContext, pt uint32, id types.Hash, kt uint16, pk []byte, hash types.Hash) *PublishInfo {
	var key []byte
	kh := common.PublicKeyWithTypeHash(kt, pk)
	key = append(key, PKDStorageTypePublisher)
	key = append(key, util.BE_Uint32ToBytes(pt)...)
	key = append(key, id[:]...)
	key = append(key, kh...)
	key = append(key, hash[:]...)
	data, err := ctx.GetStorage(contractaddress.PubKeyDistributionAddress[:], key)
	if err != nil {
		return nil
	}

	var info PubKeyInfo
	err = PublicKeyDistributionABI.UnpackVariable(&info, VariableNamePKDPublishInfo, data)
	if err != nil || !info.Valid {
		return nil
	}

	return &PublishInfo{
		Account:   info.Account,
		PType:     pt,
		PID:       id,
		KeyType:   info.KeyType,
		PubKey:    info.PubKey,
		Verifiers: info.Verifiers,
		Codes:     info.Codes,
		Fee:       info.Fee,
		Hash:      hash,
	}
}
