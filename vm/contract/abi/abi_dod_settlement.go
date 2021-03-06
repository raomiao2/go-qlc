package abi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/vm/abi"
	"github.com/qlcchain/go-qlc/vm/vmstore"
)

const (
	JsonDoDSettlement = `[
		{"type":"function","name":"DoDSettleCreateOrder","inputs":[
			{"name":"buyerAddress","type":"address"},
			{"name":"buyerName","type":"string"},
			{"name":"sellerAddress","type":"address"},
			{"name":"sellerName","type":"string"},
			{"name":"connectionName","type":"string"},
			{"name":"srcCompanyName","type":"string"},
			{"name":"srcRegion","type":"string"},
			{"name":"srcCity","type":"string"},
			{"name":"srcDataCenter","type":"string"},
			{"name":"srcPort","type":"string"},
			{"name":"dstCompanyName","type":"string"},
			{"name":"dstRegion","type":"string"},
			{"name":"dstCity","type":"string"},
			{"name":"dstDataCenter","type":"string"},
			{"name":"dstPort","type":"string"},
			{"name":"paymentType","type":"int64"},
			{"name":"billingType","type":"int64"},
			{"name":"currency","type":"string"},
			{"name":"bandwidth","type":"string"},
			{"name":"billingUnit","type":"int64"},
			{"name":"price","type":"string"},
			{"name":"startTime","type":"uint64"},
			{"name":"endTime","type":"uint64"},
			{"name":"fee","type":"string"},
			{"name":"serviceClass","type":"int64"}
		]},
		{"type":"function","name":"DoDSettleUpdateOrderInfo","inputs":[
			{"name":"buyer","type":"address"},
			{"name":"internalId","type":"hash"},
			{"name":"orderId","type":"string"},
			{"name":"productId","type":"string[]"},
			{"name":"operation","type":"string"},
			{"name":"failReason","type":"string"}
		]},
		{"type":"function","name":"DoDSettleChangeOrder","inputs":[
			{"name":"buyerAddress","type":"address"},
			{"name":"buyerName","type":"string"},
			{"name":"sellerAddress","type":"address"},
			{"name":"sellerName","type":"string"},
			{"name":"productId","type":"string"},
			{"name":"connectionName","type":"string"},
			{"name":"paymentType","type":"int64"},
			{"name":"billingType","type":"int64"},
			{"name":"currency","type":"string"},
			{"name":"bandwidth","type":"string"},
			{"name":"billingUnit","type":"int64"},
			{"name":"price","type":"string"},
			{"name":"startTime","type":"uint64"},
			{"name":"endTime","type":"uint64"},
			{"name":"fee","type":"string"},
			{"name":"serviceClass","type":"int64"}
		]},
		{"type":"function","name":"DoDSettleTerminateOrder","inputs":[
			{"name":"buyer","type":"address"},
			{"name":"orderId","type":"string"}
		]},
		{"type":"function","name":"DoDSettleUpdateProductInfo","inputs":[
			{"name":"seller","type":"address"},
			{"name":"orderId","type":"string"},
			{"name":"orderItemId","type":"string"},
			{"name":"productId","type":"string"},
			{"name":"productStatus","type":"string"}
		]}
	]`

	MethodNameDoDSettleCreateOrder       = "DoDSettleCreateOrder"
	MethodNameDoDSettleUpdateOrderInfo   = "DoDSettleUpdateOrderInfo"
	MethodNameDoDSettleChangeOrder       = "DoDSettleChangeOrder"
	MethodNameDoDSettleTerminateOrder    = "DoDSettleTerminateOrder"
	MethodNameDoDSettleUpdateProductInfo = "DoDSettleUpdateProductInfo"
)

var (
	DoDSettlementABI, _ = abi.JSONToABIContract(strings.NewReader(JsonDoDSettlement))
)

func DoDSettleBillingUnitRound(unit DoDSettleBillingUnit, s, t int64) int64 {
	to := time.Unix(t, 0)
	start := time.Unix(s, 0)
	var end time.Time

	if s == t {
		return s
	}

	switch unit {
	case DoDSettleBillingUnitYear:
		for {
			end = start.AddDate(1, 0, 0)
			if end.After(to) || end.Equal(to) {
				break
			}
		}
		return end.Unix()
	case DoDSettleBillingUnitMonth:
		for {
			end = start.AddDate(0, 1, 0)
			if end.After(to) || end.Equal(to) {
				break
			}
		}
		return end.Unix()
	case DoDSettleBillingUnitWeek:
		round := int64(60 * 60 * 24 * 7)
		return s + (t-s+round-1)/round*round
	case DoDSettleBillingUnitDay:
		round := int64(60 * 60 * 24)
		return s + (t-s+round-1)/round*round
	case DoDSettleBillingUnitHour:
		round := int64(60 * 60)
		return s + (t-s+round-1)/round*round
	case DoDSettleBillingUnitMinute:
		round := int64(60)
		return s + (t-s+round-1)/round*round
	case DoDSettleBillingUnitSecond:
		return t
	default:
		return t
	}
}

func DoDSettleCalcBillingUnit(unit DoDSettleBillingUnit, s, e int64) int {
	start := time.Unix(s, 0)
	end := time.Unix(e, 0)

	switch unit {
	case DoDSettleBillingUnitYear:
		return end.Year() - start.Year()
	case DoDSettleBillingUnitMonth:
		count := 0
		for {
			start = start.AddDate(0, 1, 0)
			count++
			if end.Sub(start) <= 0 {
				break
			}
		}
		return count
	case DoDSettleBillingUnitWeek:
		round := 60 * 60 * 24 * 7
		return int(e-s) / round
	case DoDSettleBillingUnitDay:
		round := 60 * 60 * 24
		return int(e-s) / round
	case DoDSettleBillingUnitHour:
		round := 60 * 60
		return int(e-s) / round
	case DoDSettleBillingUnitMinute:
		round := 60
		return int(e-s) / round
	case DoDSettleBillingUnitSecond:
		return int(e - s)
	default:
		return int(e - s)
	}
}

func DoDSettleCalcAmount(bs, be, s, e int64, price float64, dc *DoDSettleInvoiceConnDynamic) float64 {
	var cs, ce int64

	if s > bs {
		cs = s
	} else {
		cs = bs
	}

	if e < be {
		ce = e
	} else {
		ce = be
	}

	dc.InvoiceStartTime = cs
	dc.InvoiceEndTime = ce

	return float64(ce-cs) * price / float64(be-bs)
}

func DoDSettleCalcAdditionPrice(ns, ne int64, np float64, conn *DoDSettleConnectionInfo) (float64, error) {
	invoice, err := DoDSettleGetProductInvoice(conn, ns, ne, true, true)
	if err != nil {
		return 0, err
	}

	return np - invoice.ConnectionAmount, nil
}

func DoDSettleNeedInvoice(bs, be, s, e int64) bool {
	var cs, ce int64

	if s > bs {
		cs = s
	} else {
		cs = bs
	}

	if e < be {
		ce = e
	} else {
		ce = be
	}

	// this is overlapped zone or usage start in billing timespan
	return ce-cs > 0 || bs == e
}

func DoDSettleGetOrderInfoByInternalId(ctx *vmstore.VMContext, id types.Hash) (*DoDSettleOrderInfo, error) {
	var key []byte
	key = append(key, DoDSettleDBTableOrder)
	key = append(key, id.Bytes()...)
	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	oi := new(DoDSettleOrderInfo)
	_, err = oi.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return oi, nil
}

func DoDSettleGetInternalIdByOrderId(ctx *vmstore.VMContext, seller types.Address, orderId string) (types.Hash, error) {
	orderKey := &DoDSettleOrder{Seller: seller, OrderId: orderId}

	var key []byte
	key = append(key, DoDSettleDBTableOrderIdMap)
	key = append(key, orderKey.Hash().Bytes()...)
	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return types.ZeroHash, err
	}

	hash, err := types.BytesToHash(data)
	if err != nil {
		return types.ZeroHash, err
	}

	return hash, nil
}

func DoDSettleGetOrderInfoByOrderId(ctx *vmstore.VMContext, seller types.Address, orderId string) (*DoDSettleOrderInfo, error) {
	internalId, err := DoDSettleGetInternalIdByOrderId(ctx, seller, orderId)
	if err != nil {
		return nil, err
	}

	return DoDSettleGetOrderInfoByInternalId(ctx, internalId)
}

func DoDSettleGetConnectionInfoByProductId(ctx *vmstore.VMContext, seller types.Address, productId string) (*DoDSettleConnectionInfo, error) {
	pid := &DoDSettleProduct{Seller: seller, ProductId: productId}

	psk, err := DoDSettleGetProductStorageKeyByProductId(ctx, pid.Hash())
	if err != nil {
		return nil, fmt.Errorf("get product storage key err")
	}

	conn, err := DoDSettleGetConnectionInfoByProductStorageKey(ctx, psk)
	if err != nil {
		return nil, err
	}

	conn.ProductId = productId
	return conn, nil
}

func DoDSettleGetConnectionInfoByProductStorageKey(ctx *vmstore.VMContext, hash types.Hash) (*DoDSettleConnectionInfo, error) {
	var key []byte
	key = append(key, DoDSettleDBTableProduct)
	key = append(key, hash.Bytes()...)
	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	conn := new(DoDSettleConnectionInfo)
	_, err = conn.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	pd, _ := DoDSettleGetProductIdByStorageKey(ctx, hash)
	if pd != nil {
		conn.ProductId = pd.ProductId
	}

	if len(conn.ProductId) > 0 {
		if conn.Active != nil {
			ts, _ := DoDSettleGetPAYGTimeSpan(ctx, conn.ProductId, conn.Active.OrderId)
			if ts != nil {
				if conn.Active.StartTime == 0 {
					conn.Active.StartTime = ts.StartTime
				}

				if conn.Active.EndTime == 0 {
					conn.Active.EndTime = ts.EndTime
				}
			}
		}

		for _, d := range conn.Done {
			ts, _ := DoDSettleGetPAYGTimeSpan(ctx, conn.ProductId, d.OrderId)
			if ts != nil {
				if d.StartTime == 0 {
					d.StartTime = ts.StartTime
				}

				if d.EndTime == 0 {
					d.EndTime = ts.EndTime
				}
			}
		}
	}

	return conn, nil
}

func DoDSettleSetProductStorageKeyByProductId(ctx *vmstore.VMContext, psk, pid types.Hash) error {
	var key []byte
	key = append(key, DoDSettleDBTableProductToOrder)
	key = append(key, pid.Bytes()...)
	err := ctx.SetStorage(nil, key, psk.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleGetProductStorageKeyByProductId(ctx *vmstore.VMContext, pid types.Hash) (types.Hash, error) {
	var key []byte
	key = append(key, DoDSettleDBTableProductToOrder)
	key = append(key, pid.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return types.ZeroHash, err
	}

	hash, err := types.BytesToHash(data)
	if err != nil {
		return types.ZeroHash, err
	}

	return hash, nil
}

func DoDSettleSetProductIdByStorageKey(ctx *vmstore.VMContext, psk types.Hash, productId string, seller types.Address) error {
	pi := &DoDSettleProduct{Seller: seller, ProductId: productId}
	data, err := pi.MarshalMsg(nil)
	if err != nil {
		return err
	}

	var key []byte
	key = append(key, DoDSettleDBTableOrderToProduct)
	key = append(key, psk.Bytes()...)
	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleGetProductIdByStorageKey(ctx *vmstore.VMContext, psk types.Hash) (*DoDSettleProduct, error) {
	var key []byte
	key = append(key, DoDSettleDBTableOrderToProduct)
	key = append(key, psk.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	pi := new(DoDSettleProduct)
	_, err = pi.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return pi, nil
}

func DoDSettleUpdateOrder(ctx *vmstore.VMContext, order *DoDSettleOrderInfo, id types.Hash) error {
	data, err := order.MarshalMsg(nil)
	if err != nil {
		return err
	}

	var key []byte
	key = append(key, DoDSettleDBTableOrder)
	key = append(key, id.Bytes()...)
	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleUpdateConnection(ctx *vmstore.VMContext, conn *DoDSettleConnectionInfo, id types.Hash) error {
	data, err := conn.MarshalMsg(nil)
	if err != nil {
		return err
	}

	var key []byte
	key = append(key, DoDSettleDBTableProduct)
	key = append(key, id.Bytes()...)
	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleUpdateConnectionRawParam(ctx *vmstore.VMContext, param *DoDSettleConnectionParam, id types.Hash) error {
	var key []byte
	key = append(key, DoDSettleDBTableConnRawParam)
	key = append(key, id.Bytes()...)

	cp := new(DoDSettleConnectionRawParam)

	data, _ := ctx.GetStorage(nil, key)
	if len(data) > 0 {
		_, err := cp.UnmarshalMsg(data)
		if err != nil {
			return err
		}

		if len(param.ItemId) > 0 {
			cp.ItemId = param.ItemId
		}

		if len(param.ConnectionName) > 0 {
			cp.ConnectionName = param.ConnectionName
		}

		if param.PaymentType != DoDSettlePaymentTypeNull {
			cp.PaymentType = param.PaymentType
		}

		if param.BillingType != DoDSettleBillingTypeNull {
			cp.BillingType = param.BillingType
		}

		if len(param.Currency) > 0 {
			cp.Currency = param.Currency
		}

		if param.ServiceClass != DoDSettleServiceClassNull {
			cp.ServiceClass = param.ServiceClass
		}

		if len(param.Bandwidth) > 0 {
			cp.Bandwidth = param.Bandwidth
		}

		if param.BillingUnit != DoDSettleBillingUnitNull {
			cp.BillingUnit = param.BillingUnit
		}

		if param.Price > 0 {
			cp.Price = param.Price
		}

		if param.StartTime > 0 {
			cp.StartTime = param.StartTime
		}

		if param.EndTime > 0 {
			cp.EndTime = param.EndTime
		}
	} else {
		cp.ItemId = param.ItemId
		cp.BuyerProductId = param.BuyerProductId
		cp.ProductOfferingId = param.ProductOfferingId
		cp.SrcCompanyName = param.SrcCompanyName
		cp.SrcRegion = param.SrcRegion
		cp.SrcCity = param.SrcCity
		cp.SrcDataCenter = param.SrcDataCenter
		cp.SrcPort = param.SrcPort
		cp.DstCompanyName = param.DstCompanyName
		cp.DstRegion = param.DstRegion
		cp.DstCity = param.DstCity
		cp.DstDataCenter = param.DstDataCenter
		cp.DstPort = param.DstPort
		cp.ConnectionName = param.ConnectionName
		cp.PaymentType = param.PaymentType
		cp.BillingType = param.BillingType
		cp.Currency = param.Currency
		cp.ServiceClass = param.ServiceClass
		cp.Bandwidth = param.Bandwidth
		cp.BillingUnit = param.BillingUnit
		cp.Price = param.Price
		cp.StartTime = param.StartTime
		cp.EndTime = param.EndTime
	}

	data, err := cp.MarshalMsg(nil)
	if err != nil {
		return err
	}

	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleGetConnectionRawParam(ctx *vmstore.VMContext, id types.Hash) (*DoDSettleConnectionRawParam, error) {
	var key []byte
	key = append(key, DoDSettleDBTableConnRawParam)
	key = append(key, id.Bytes()...)

	cp := new(DoDSettleConnectionRawParam)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	_, err = cp.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return cp, nil
}

func DoDSettleInheritRawParam(src *DoDSettleConnectionRawParam, dst *DoDSettleConnectionParam) {
	if len(dst.ConnectionName) == 0 {
		dst.ConnectionName = src.ConnectionName
	}

	if len(dst.Currency) == 0 {
		dst.Currency = src.Currency
	}

	if dst.BillingType == 0 {
		dst.BillingType = src.BillingType
	}

	if dst.BillingUnit == 0 {
		dst.BillingUnit = src.BillingUnit
	}

	if len(dst.Bandwidth) == 0 {
		dst.Bandwidth = src.Bandwidth
	}

	if dst.ServiceClass == 0 {
		dst.ServiceClass = src.ServiceClass
	}

	if dst.PaymentType == 0 {
		dst.PaymentType = src.PaymentType
	}
}

func DoDSettleInheritParam(src, dst *DoDSettleConnectionDynamicParam) {
	if len(dst.ConnectionName) == 0 {
		dst.ConnectionName = src.ConnectionName
	}

	if len(dst.Currency) == 0 {
		dst.Currency = src.Currency
	}

	if dst.BillingType == 0 {
		dst.BillingType = src.BillingType
	}

	if dst.BillingUnit == 0 {
		dst.BillingUnit = src.BillingUnit
	}

	if len(dst.Bandwidth) == 0 {
		dst.Bandwidth = src.Bandwidth
	}

	if dst.ServiceClass == 0 {
		dst.ServiceClass = src.ServiceClass
	}

	if dst.PaymentType == 0 {
		dst.PaymentType = src.PaymentType
	}
}

func DoDSettleGetInternalIdListByAddress(ctx *vmstore.VMContext, address types.Address) ([]types.Hash, error) {
	var key []byte
	key = append(key, DoDSettleDBTableUser)
	key = append(key, address.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	userInfo := new(DoDSettleUserInfos)
	_, err = userInfo.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	hs := make([]types.Hash, 0)
	for _, i := range userInfo.InternalIds {
		hs = append(hs, i.InternalId)
	}

	return hs, nil
}

func DoDSettleGetProductIdListByAddress(ctx *vmstore.VMContext, address types.Address) ([]*DoDSettleProduct, error) {
	var key []byte
	key = append(key, DoDSettleDBTableUserProduct)
	key = append(key, address.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	up := new(DoDSettleUserProducts)
	_, err = up.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return up.Products, nil
}

func DoDSettleGetOrderIdListByAddress(ctx *vmstore.VMContext, address types.Address) ([]*DoDSettleOrder, error) {
	var key []byte
	key = append(key, DoDSettleDBTableUser)
	key = append(key, address.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	userInfo := new(DoDSettleUserInfos)
	_, err = userInfo.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return userInfo.OrderIds, nil
}

func DoDSettleCalcConnInvoice(conn *DoDSettleConnectionInfo, order *DoDSettleOrderInfo, start, end, now int64, flight,
	split bool) *DoDSettleInvoiceConnDetail {

	ic := &DoDSettleInvoiceConnDetail{
		DoDSettleConnectionStaticParam: DoDSettleConnectionStaticParam{
			ProductId:      conn.ProductId,
			SrcCompanyName: conn.SrcCompanyName,
			SrcRegion:      conn.SrcRegion,
			SrcCity:        conn.SrcCity,
			SrcDataCenter:  conn.SrcDataCenter,
			SrcPort:        conn.SrcPort,
			DstCompanyName: conn.DstCompanyName,
			DstRegion:      conn.DstRegion,
			DstCity:        conn.DstCity,
			DstDataCenter:  conn.DstDataCenter,
			DstPort:        conn.DstPort,
		},
		Usage: make([]*DoDSettleInvoiceConnDynamic, 0),
	}

	if conn.Active != nil {
		conn.Done = append(conn.Done, conn.Active)
	}

	for _, done := range conn.Done {
		if order != nil && done.OrderId != order.OrderId {
			continue
		}

		de := done.EndTime
		if de == 0 {
			de = end
		}

		if DoDSettleNeedInvoice(done.StartTime, de, start, end) {
			dc := &DoDSettleInvoiceConnDynamic{
				DoDSettleConnectionDynamicParam: DoDSettleConnectionDynamicParam{
					OrderId:        done.OrderId,
					ConnectionName: done.ConnectionName,
					PaymentType:    done.PaymentType,
					BillingType:    done.BillingType,
					Currency:       done.Currency,
					ServiceClass:   done.ServiceClass,
					Bandwidth:      done.Bandwidth,
					BillingUnit:    done.BillingUnit,
					Price:          done.Price,
					Addition:       done.Addition,
					StartTime:      done.StartTime,
					EndTime:        done.EndTime,
				},
			}

			for _, t := range conn.Track {
				if dc.OrderId == t.OrderId {
					dc.OrderType = t.OrderType
					break
				}
			}

			if done.BillingType == DoDSettleBillingTypeDOD {
				if flight {
					if split {
						dc.Amount = DoDSettleCalcAmount(done.StartTime, done.EndTime, start, end, done.Addition, dc)
					} else {
						if done.StartTime >= start {
							dc.Amount = done.Addition
							dc.InvoiceStartTime = done.StartTime
							dc.InvoiceEndTime = done.EndTime
						}
					}
				} else {
					if now >= done.EndTime {
						dc.Amount = DoDSettleCalcAmount(done.StartTime, done.EndTime, start, end, done.Addition, dc)
					}
				}
			} else {
				if done.StartTime == 0 {
					continue
				}

				if done.EndTime == 0 {
					if !flight || !split {
						continue
					}
					done.EndTime = end
				}

				if split {
					if start >= done.StartTime {
						dc.InvoiceStartTime = DoDSettleBillingUnitRound(done.BillingUnit, done.StartTime, start)
					} else {
						dc.InvoiceStartTime = done.StartTime
					}

					if end <= done.EndTime {
						dc.InvoiceEndTime = DoDSettleBillingUnitRound(done.BillingUnit, done.StartTime, end)
					} else {
						dc.InvoiceEndTime = done.EndTime
					}

					dc.InvoiceUnitCount = DoDSettleCalcBillingUnit(done.BillingUnit, dc.InvoiceStartTime, dc.InvoiceEndTime)
					dc.Amount = done.Price * float64(dc.InvoiceUnitCount)
				} else {
					if done.StartTime >= start && done.StartTime < end {
						dc.InvoiceStartTime = done.StartTime
						dc.InvoiceEndTime = done.EndTime

						dc.InvoiceUnitCount = DoDSettleCalcBillingUnit(done.BillingUnit, dc.InvoiceStartTime, dc.InvoiceEndTime)
						dc.Amount = done.Price * float64(dc.InvoiceUnitCount)
					}
				}
			}

			dc.StartTimeStr = time.Unix(dc.StartTime, 0).String()
			dc.EndTimeStr = time.Unix(dc.EndTime, 0).String()
			dc.InvoiceStartTimeStr = time.Unix(dc.InvoiceStartTime, 0).String()
			dc.InvoiceEndTimeStr = time.Unix(dc.InvoiceEndTime, 0).String()

			ic.ConnectionAmount += dc.Amount
			ic.Usage = append(ic.Usage, dc)
		}
	}

	if conn.Disconnect != nil && ((conn.Disconnect.DisconnectAt == start) ||
		(conn.Disconnect.DisconnectAt > start && conn.Disconnect.DisconnectAt < end)) {
		if (order != nil && conn.Disconnect.OrderId == order.OrderId) || order == nil {
			dc := &DoDSettleInvoiceConnDynamic{
				DoDSettleConnectionDynamicParam: DoDSettleConnectionDynamicParam{
					OrderId:   conn.Disconnect.OrderId,
					Currency:  conn.Disconnect.Currency,
					Price:     conn.Disconnect.Price,
					StartTime: conn.Disconnect.DisconnectAt,
					EndTime:   conn.Disconnect.DisconnectAt,
				},
				InvoiceStartTime: conn.Disconnect.DisconnectAt,
				InvoiceEndTime:   conn.Disconnect.DisconnectAt,
				Amount:           conn.Disconnect.Price,
			}

			for _, t := range conn.Track {
				if dc.OrderId == t.OrderId {
					dc.OrderType = t.OrderType
					break
				}
			}

			dc.StartTimeStr = time.Unix(dc.StartTime, 0).String()
			dc.EndTimeStr = time.Unix(dc.EndTime, 0).String()
			dc.InvoiceStartTimeStr = time.Unix(dc.InvoiceStartTime, 0).String()
			dc.InvoiceEndTimeStr = time.Unix(dc.InvoiceEndTime, 0).String()

			ic.ConnectionAmount += dc.Amount
			ic.Usage = append(ic.Usage, dc)
		}
	}

	return ic
}

func DoDSettleGetOrderInvoice(ctx *vmstore.VMContext, seller types.Address, order *DoDSettleOrderInfo, start, end int64,
	flight, split bool) (*DoDSettleInvoiceOrderDetail, error) {

	now := time.Now().Unix()
	invoiceOrder := new(DoDSettleInvoiceOrderDetail)
	invoiceOrder.OrderId = order.OrderId
	invoiceOrder.Connections = make([]*DoDSettleInvoiceConnDetail, 0)

	internalId, err := DoDSettleGetInternalIdByOrderId(ctx, seller, order.OrderId)
	if err != nil {
		return nil, fmt.Errorf("get internal id err %s", err)
	}

	invoiceOrder.InternalId = internalId

	for _, c := range order.Connections {
		var conn *DoDSettleConnectionInfo

		if len(c.ProductId) > 0 {
			conn, _ = DoDSettleGetConnectionInfoByProductId(ctx, seller, c.ProductId)
		} else {
			otp := &DoDSettleOrderToProduct{Seller: order.Seller.Address, OrderId: order.OrderId, OrderItemId: c.OrderItemId}
			conn, _ = DoDSettleGetConnectionInfoByProductStorageKey(ctx, otp.Hash())

			pi, _ := DoDSettleGetProductIdByStorageKey(ctx, otp.Hash())
			if pi != nil {
				conn.ProductId = pi.ProductId
			}
		}

		if conn == nil {
			continue
		}

		ic := DoDSettleCalcConnInvoice(conn, order, start, end, now, flight, split)

		invoiceOrder.OrderAmount += ic.ConnectionAmount
		invoiceOrder.ConnectionCount++
		invoiceOrder.Connections = append(invoiceOrder.Connections, ic)
	}

	return invoiceOrder, nil
}

func DoDSettleGetProductInvoice(conn *DoDSettleConnectionInfo, start, end int64, flight, split bool) (*DoDSettleInvoiceConnDetail, error) {
	now := time.Now().Unix()
	return DoDSettleCalcConnInvoice(conn, nil, start, end, now, flight, split), nil
}

func DoDSettleGenerateInvoiceByOrder(ctx *vmstore.VMContext, seller types.Address, orderId string, start, end int64,
	flight, split bool) (*DoDSettleOrderInvoice, error) {

	invoice := new(DoDSettleOrderInvoice)

	if start < 0 || end < 0 || start > end {
		return nil, fmt.Errorf("invalid start or end time")
	}

	order, err := DoDSettleGetOrderInfoByOrderId(ctx, seller, orderId)
	if err != nil {
		return nil, err
	}

	invoiceOrder, err := DoDSettleGetOrderInvoice(ctx, seller, order, start, end, flight, split)
	if err != nil {
		return nil, err
	}

	invoice.Flight = flight
	invoice.Split = split
	invoice.StartTime = start
	invoice.EndTime = end
	invoice.Currency = order.Connections[0].Currency
	invoice.Buyer = order.Buyer
	invoice.Seller = order.Seller
	invoice.TotalConnectionCount = invoiceOrder.ConnectionCount
	invoice.TotalAmount = invoiceOrder.OrderAmount
	invoice.Order = invoiceOrder

	data, _ := json.Marshal(invoice)
	invoice.InvoiceId = types.HashData(data)

	return invoice, nil
}

func DoDSettleGenerateInvoiceByProduct(ctx *vmstore.VMContext, seller types.Address, productId string, start, end int64,
	flight, split bool) (*DoDSettleProductInvoice, error) {
	invoice := new(DoDSettleProductInvoice)

	if start < 0 || end < 0 || start > end {
		return nil, fmt.Errorf("invalid start or end time")
	}

	conn, err := DoDSettleGetConnectionInfoByProductId(ctx, seller, productId)
	if err != nil {
		return nil, fmt.Errorf("get product info err")
	}

	order, err := DoDSettleGetOrderInfoByOrderId(ctx, seller, conn.Track[0].OrderId)
	if err != nil {
		return nil, fmt.Errorf("get order info err %s", conn.Track[0].OrderId)
	}

	productOrder, err := DoDSettleGetProductInvoice(conn, start, end, flight, split)
	if err != nil {
		return nil, err
	}

	for _, u := range productOrder.Usage {
		internalId, err := DoDSettleGetInternalIdByOrderId(ctx, seller, u.OrderId)
		if err != nil {
			return nil, err
		}

		u.InternalId = internalId.String()
	}

	invoice.Flight = flight
	invoice.Split = split
	invoice.StartTime = start
	invoice.EndTime = end
	invoice.Currency = order.Connections[0].Currency
	invoice.Buyer = order.Buyer
	invoice.Seller = order.Seller
	invoice.TotalAmount = productOrder.ConnectionAmount
	invoice.Connection = productOrder

	data, _ := json.Marshal(invoice)
	invoice.InvoiceId = types.HashData(data)

	return invoice, nil
}

func DoDSettleGenerateInvoiceByBuyer(ctx *vmstore.VMContext, seller, buyer types.Address, start, end int64, flight,
	split bool) (*DoDSettleBuyerInvoice, error) {

	invoice := new(DoDSettleBuyerInvoice)

	if start < 0 || end < 0 || start > end {
		return nil, fmt.Errorf("invalid start or end time")
	}

	invoice.Flight = flight
	invoice.Split = split
	invoice.StartTime = start
	invoice.EndTime = end
	invoice.Orders = make([]*DoDSettleInvoiceOrderDetail, 0)

	orders, err := DoDSettleGetOrderIdListByAddress(ctx, buyer)
	if err != nil {
		return nil, err
	}

	productIdMap := make(map[string]struct{})

	for _, o := range orders {
		order, err := DoDSettleGetOrderInfoByOrderId(ctx, seller, o.OrderId)
		if err != nil {
			return nil, err
		}

		if invoice.Buyer == nil {
			invoice.Currency = order.Connections[0].Currency
			invoice.Buyer = order.Buyer
			invoice.Seller = order.Seller
		}

		invoiceOrder, err := DoDSettleGetOrderInvoice(ctx, seller, order, start, end, flight, split)
		if err != nil {
			return nil, err
		}

		if invoiceOrder.OrderAmount == 0 {
			continue
		}

		for _, c := range invoiceOrder.Connections {
			productIdMap[c.ProductId] = struct{}{}
		}

		invoice.OrderCount++
		invoice.TotalAmount += invoiceOrder.OrderAmount
		invoice.Orders = append(invoice.Orders, invoiceOrder)
	}

	invoice.TotalConnectionCount = len(productIdMap)

	data, _ := json.Marshal(invoice)
	invoice.InvoiceId = types.HashData(data)

	return invoice, nil
}

func DoDSettleSetSellerConnectionActive(ctx *vmstore.VMContext, active *DoDSettleConnectionActive, id types.Hash) error {
	data, err := active.MarshalMsg(nil)
	if err != nil {
		return err
	}

	var key []byte
	key = append(key, DoDSettleDBTableSellerConnActive)
	key = append(key, id.Bytes()...)
	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleGetSellerConnectionActive(ctx *vmstore.VMContext, id types.Hash) (*DoDSettleConnectionActive, error) {
	var key []byte
	key = append(key, DoDSettleDBTableSellerConnActive)
	key = append(key, id.Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	act := new(DoDSettleConnectionActive)
	_, err = act.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return act, nil
}

func DoDSettleUpdatePAYGTimeSpan(ctx *vmstore.VMContext, productId, orderId string, st, et int64) error {
	tsk := DoDSettlePAYGTimeSpanKey{ProductId: productId, OrderId: orderId}
	ts := new(DoDSettlePAYGTimeSpan)

	ots, _ := DoDSettleGetPAYGTimeSpan(ctx, productId, orderId)
	if ots != nil {
		ts.StartTime = ots.StartTime
		ts.EndTime = ots.EndTime

		if st > 0 {
			ts.StartTime = st
		}

		if et > 0 {
			ts.EndTime = et
		}
	} else {
		ts.StartTime = st
		ts.EndTime = et
	}

	data, err := ts.MarshalMsg(nil)
	if err != nil {
		return err
	}

	var key []byte
	key = append(key, DoDSettleDBTablePAYGTimeSpan)
	key = append(key, tsk.Hash().Bytes()...)
	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}

func DoDSettleGetPAYGTimeSpan(ctx *vmstore.VMContext, productId, orderId string) (*DoDSettlePAYGTimeSpan, error) {
	tsk := DoDSettlePAYGTimeSpanKey{ProductId: productId, OrderId: orderId}
	tk := new(DoDSettlePAYGTimeSpan)

	var key []byte
	key = append(key, DoDSettleDBTablePAYGTimeSpan)
	key = append(key, tsk.Hash().Bytes()...)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		return nil, err
	}

	_, err = tk.UnmarshalMsg(data)
	if err != nil {
		return nil, err
	}

	return tk, nil
}

func DoDSettleUpdateUserProduct(ctx *vmstore.VMContext, buyer, seller types.Address, productId string) error {
	var key []byte
	key = append(key, DoDSettleDBTableUserProduct)
	key = append(key, buyer.Bytes()...)

	up := new(DoDSettleUserProducts)

	data, err := ctx.GetStorage(nil, key)
	if err != nil {
		up.Products = make([]*DoDSettleProduct, 0)

		product := &DoDSettleProduct{Seller: seller, ProductId: productId}
		up.Products = append(up.Products, product)
	} else {
		_, err = up.UnmarshalMsg(data)
		if err != nil {
			return err
		}

		for _, p := range up.Products {
			if p.ProductId == productId {
				return nil
			}
		}

		product := &DoDSettleProduct{Seller: seller, ProductId: productId}
		up.Products = append(up.Products, product)
	}

	data, err = up.MarshalMsg(nil)
	if err != nil {
		return err
	}

	err = ctx.SetStorage(nil, key, data)
	if err != nil {
		return err
	}

	return nil
}
