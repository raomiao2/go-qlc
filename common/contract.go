package common

import (
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
)

const (
	OracleTypeEmail uint32 = iota
	OracleTypeWeChat
	OracleTypeInvalid
)

const RandomCodeLen = 16

type ContractGapType byte

const (
	ContractNoGap ContractGapType = iota
	ContractRewardGapPov
	ContractDPKIGapPublish
	ContractPermGapAdmin
	ContractDoDOrderState
)

const (
	OracleExpirePovHeight  = 10
	OracleVerifyMinAccount = 3
	OracleVerifyMaxAccount = 5
)

var (
	MinVerifierPledgeAmount = types.NewBalance(3e+14) // 3M
	OracleCost              = types.NewBalance(1e+7)  // 0.1
	PublishCost             = types.NewBalance(5e+8)  // 5
)

func OracleStringToType(os string) uint32 {
	switch os {
	case "email":
		return OracleTypeEmail
	case "weChat":
		return OracleTypeWeChat
	default:
		return OracleTypeInvalid
	}
}

func OracleTypeToString(ot uint32) string {
	switch ot {
	case OracleTypeEmail:
		return "email"
	case OracleTypeWeChat:
		return "weChat"
	default:
		return "invalid"
	}
}

const (
	VerifierMinNum = 1
	VerifierMaxNum = 5
)

const (
	PublicKeyTypeED25519 uint16 = iota
	PublicKeyTypeRSA4096
	PublicKeyTypeInvalid
)

func PublicKeyTypeFromString(t string) uint16 {
	switch t {
	case "ed25519":
		return PublicKeyTypeED25519
	case "rsa4096":
		return PublicKeyTypeRSA4096
	default:
		return PublicKeyTypeInvalid
	}
}

func PublicKeyTypeToString(t uint16) string {
	switch t {
	case PublicKeyTypeED25519:
		return "ed25519"
	case PublicKeyTypeRSA4096:
		return "rsa4096"
	default:
		return "invalid"
	}
}

func PublicKeyWithTypeHash(t uint16, k []byte) []byte {
	d := make([]byte, 0)
	d = append(d, util.BE_Uint16ToBytes(t)...)
	d = append(d, k...)

	h, _ := types.Sha256HashData(d)
	return h.Bytes()
}

const (
	PtmKeyVBtypeDefault uint16 = iota
	PtmKeyVBtypeA2p
	PtmKeyVBtypeDod
	PtmKeyVBtypeCloud
	PtmKeyVBtypeInvaild
)

const (
	PtmKeyVBtypeStrDefault = "default"
	PtmKeyVBtypeStrA2p     = "a2p"
	PtmKeyVBtypeStrDod     = "dod"
	PtmKeyVBtypeStrCloud   = "cloud"
	PtmKeyVBtypeStrInvaild = "invalid"
)

func PtmKeyBtypeFromString(t string) uint16 {
	switch t {
	case PtmKeyVBtypeStrDefault:
		return PtmKeyVBtypeDefault
	case PtmKeyVBtypeStrA2p:
		return PtmKeyVBtypeA2p
	case PtmKeyVBtypeStrDod:
		return PtmKeyVBtypeDod
	case PtmKeyVBtypeStrCloud:
		return PtmKeyVBtypeCloud
	default:
		return PtmKeyVBtypeInvaild
	}
}

func PtmKeyBtypeToString(t uint16) string {
	switch t {
	case PtmKeyVBtypeDefault:
		return PtmKeyVBtypeStrDefault
	case PtmKeyVBtypeA2p:
		return PtmKeyVBtypeStrA2p
	case PtmKeyVBtypeDod:
		return PtmKeyVBtypeStrDod
	case PtmKeyVBtypeCloud:
		return PtmKeyVBtypeStrCloud
	default:
		return PtmKeyVBtypeStrInvaild
	}
}
