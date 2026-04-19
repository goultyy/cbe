package cbe

import (
	"database/sql"
)

// Main matching engine, takes share order and attempts to process and match it
//
// The error here is not for a lack of matching instead for problems with data, all problems relating to matching
// are reflected in ShareOrder.Status and ShareOrder.Quantity. We also don't do validation on data for the order
// so we assume that data is all correct.
func (so ShareOrder) MatchShareOrder() (Share, error) {
	market, _ := GetMarket(so.MarketID)
	original_quantity := so.Quantity

	// take a snapshot of all available trades on the same 'yes/no' and opposite direction
	shares, err := market.GetShareOrdersByDirection(so.Direction, OrderDirection(1-int(so.BuySell)))
	if err == sql.ErrNoRows {
		// nothing to match, so if we have FOK or IOC we kill, else we leave it for later
		if so.ForceType == ORDER_FORCE_FOK || so.ForceType == ORDER_FORCE_IOC {
			so.UpdateStatus(ORDER_STATUS_CANCELLED)
			return Share{}, nil
		} else {
			so.UpdateStatus(ORDER_STATUS_PENDING)
			return Share{}, nil
		}
	} else if err != nil {
		return Share{}, err
	}

	price_paid := int64(0)

	// use this to dictate what to adjust, but do not do it yet, as we may not need to make a new share
	var need_to_adjust []MarketMatchAdjustment

	// we have some shares to match with, we will try to match with the best price first, which is the first one in the list
	for _, share := range shares {
		if so.Quantity == 0 {
			break
		}
		// if we have a market order, we will fill as much as possible at the best price, and then move on to the next best price until we have filled all or we have no more shares to match with
		if so.Type == ORDER_TYPE_MARKET {
			if share.Quantity <= so.Quantity {
				// we take all of the share
				so.Quantity -= share.Quantity
				price_paid += int64(share.BestPrice) * share.Quantity
				need_to_adjust = append(need_to_adjust, MarketMatchAdjustment{MarketID: so.MarketID, OrderID: share.OrderID, NewQuantity: 0})
			} else {
				// take a bit of it, and fill our entire order
				need_to_adjust = append(need_to_adjust, MarketMatchAdjustment{MarketID: so.MarketID, OrderID: share.OrderID, NewQuantity: share.Quantity - so.Quantity})
				price_paid += int64(share.BestPrice) * so.Quantity
				so.Quantity = 0
			}
		} else {
			// if we have a limit order, we will only fill if the price is better than or equal to our limit price, for buy orders, better means lower, for sell orders, better means higher
			if (so.BuySell == ORDER_BUY && share.BestPrice <= so.BestPrice) || (so.BuySell == ORDER_SELL && share.BestPrice >= so.BestPrice) {
				if share.Quantity <= so.Quantity {
					// same thing, take the entire share
					so.Quantity -= share.Quantity
					price_paid += int64(share.BestPrice) * share.Quantity
					need_to_adjust = append(need_to_adjust, MarketMatchAdjustment{MarketID: so.MarketID, OrderID: share.OrderID, NewQuantity: 0})
				} else {
					// take a bit
					need_to_adjust = append(need_to_adjust, MarketMatchAdjustment{MarketID: so.MarketID, OrderID: share.OrderID, NewQuantity: share.Quantity - so.Quantity})
					price_paid += int64(share.BestPrice) * so.Quantity
					so.Quantity = 0
				}
			} else {
				// we have reached a price that is not better than our limit price, so we stop trying to match
				break
			}
		}
	}

	if so.Quantity != 0 && (so.Quantity < original_quantity) {
		// We've filled some but not all
		if so.ForceType == ORDER_FORCE_FOK { // Therefore cancel FOK and leave GTC & IOC
			so.UpdateStatus(ORDER_STATUS_CANCELLED)
			return Share{}, nil
		} else {
			so.UpdateStatus(ORDER_STATUS_PARTIALLY_FILLED)
		}
	} else if so.Quantity == 0 {
		so.UpdateStatus(ORDER_STATUS_FILLED)
	} else {
		// failed to fill any of the order
		if so.ForceType == ORDER_FORCE_GTC {
			so.UpdateStatus(ORDER_STATUS_PENDING) // leave gtc
		} else {
			so.UpdateStatus(ORDER_STATUS_CANCELLED) // else kill it
			return Share{}, nil
		}
	}

	for _, v := range need_to_adjust {
		share, _ := market.GetShareOrder(v.OrderID)
		err := share.ReduceQuantity(v.NewQuantity)
		if err != nil {
			return Share{}, err
		}
	}

	so.ReduceQuantity(so.Quantity)

	share_id, err := CreateShare(Share{
		MarketID:           so.MarketID,
		OrderID:            so.OrderID,
		Direction:          so.Direction,
		Price:              USDCBaseAmount(price_paid),
		Quantity:           original_quantity - so.Quantity,
		Key:                ShareKey(NewGenericSecureKey()),
		TimestampRequested: so.TimestampRequested,
		TimestampFulfilled: NewTimestamp(),
	})

	if err != nil {
		return Share{}, err
	} else {
		share, _ := market.GetShare(share_id)
		return share, nil
	}
}
