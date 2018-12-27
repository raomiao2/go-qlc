package protos

import (
	"github.com/gogo/protobuf/proto"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/p2p/protos/pb"
)

type ConfirmReqBlock struct {
	Blk types.Block
}

// ToProto converts domain ConfirmReqBlock into proto ConfirmReqBlock
func ConfirmReqBlockToProto(confirmReq *ConfirmReqBlock) ([]byte, error) {
	blkData, err := confirmReq.Blk.MarshalMsg(nil)
	if err != nil {
		return nil, err
	}
	blockType := confirmReq.Blk.GetType()
	bppb := &pb.PublishBlock{
		Blocktype: uint32(blockType),
		Block:     blkData,
	}
	data, err := proto.Marshal(bppb)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ConfirmReqBlockFromProto parse the data into ConfirmReqBlock message
func ConfirmReqBlockFromProto(data []byte) (*ConfirmReqBlock, error) {
	bp := new(pb.ConfirmReq)
	if err := proto.Unmarshal(data, bp); err != nil {
		logger.Error("Failed to unmarshal BulkPullRspPacket message.")
		return nil, err
	}
	blockType := bp.Blocktype
	blk, err := types.NewBlock(types.BlockType(blockType))
	if err != nil {
		return nil, err
	}
	if _, err = blk.UnmarshalMsg(bp.Block); err != nil {
		return nil, err
	}
	confirmReqBlock := &ConfirmReqBlock{
		Blk: blk,
	}
	return confirmReqBlock, nil
}
