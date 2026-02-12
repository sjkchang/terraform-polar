// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polarsource/polar-go/models/components"
)

// --- Build SDK Create request ---

func buildProductCreateRequest(ctx context.Context, data *ProductResourceModel) (*components.ProductCreate, diag.Diagnostics) {
	var diags diag.Diagnostics

	name := data.Name.ValueString()

	var description *string
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		d := data.Description.ValueString()
		description = &d
	}

	var medias []string
	if !data.Medias.IsNull() && !data.Medias.IsUnknown() {
		d := data.Medias.ElementsAs(ctx, &medias, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !data.RecurringInterval.IsNull() && !data.RecurringInterval.IsUnknown() {
		// Recurring product
		prices, d := pricesToRecurringCreateSDK(data.Prices)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		recurring := components.ProductCreateRecurring{
			Name:              name,
			Description:       description,
			Prices:            prices,
			RecurringInterval: components.SubscriptionRecurringInterval(data.RecurringInterval.ValueString()),
			Medias:            medias,
		}

		if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
			m, d := metadataToCreateSDK(ctx, data.Metadata, components.CreateProductCreateRecurringMetadataStr)
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			recurring.Metadata = m
		}

		result := components.CreateProductCreateProductCreateRecurring(recurring)
		return &result, diags
	}

	// One-time product
	prices, d := pricesToOneTimeCreateSDK(data.Prices)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	oneTime := components.ProductCreateOneTime{
		Name:        name,
		Description: description,
		Prices:      prices,
		Medias:      medias,
	}

	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		m, d := metadataToCreateSDK(ctx, data.Metadata, components.CreateProductCreateOneTimeMetadataStr)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		oneTime.Metadata = m
	}

	result := components.CreateProductCreateProductCreateOneTime(oneTime)
	return &result, diags
}

// --- Build SDK Update request ---

func buildProductUpdateRequest(ctx context.Context, data *ProductResourceModel, currentPrices []components.Prices) (*components.ProductUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics

	name := data.Name.ValueString()
	update := components.ProductUpdate{
		Name: &name,
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		d := data.Description.ValueString()
		update.Description = &d
	}

	prices, d := pricesToUpdateSDK(data.Prices, currentPrices)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	update.Prices = prices

	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		m, d := metadataToCreateSDK(ctx, data.Metadata, components.CreateProductUpdateMetadataStr)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		update.Metadata = m
	}

	if !data.Medias.IsNull() && !data.Medias.IsUnknown() {
		var medias []string
		d := data.Medias.ElementsAs(ctx, &medias, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		update.Medias = medias
	}

	isArchived := data.IsArchived.ValueBool()
	update.IsArchived = &isArchived

	return &update, diags
}

// --- Map SDK response to Terraform state ---

func mapProductResponseToState(ctx context.Context, product *components.Product, data *ProductResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(product.ID)
	data.Name = types.StringValue(product.Name)
	data.IsArchived = types.BoolValue(product.IsArchived)
	data.Description = optionalStringValue(product.Description)

	if product.RecurringInterval != nil {
		data.RecurringInterval = types.StringValue(string(*product.RecurringInterval))
	} else {
		data.RecurringInterval = types.StringNull()
	}

	// Map prices
	data.Prices = sdkPricesToModel(product.Prices)

	// Map metadata
	data.Metadata = sdkProductMetadataToMap(ctx, product.Metadata, diags)

	// Map medias
	if len(product.Medias) > 0 {
		mediaIDs := make([]string, len(product.Medias))
		for i, m := range product.Medias {
			mediaIDs[i] = m.ID
		}
		mediaList, d := types.ListValueFrom(ctx, types.StringType, mediaIDs)
		diags.Append(d...)
		data.Medias = mediaList
	} else {
		data.Medias = types.ListNull(types.StringType)
	}
}

// --- Price conversion helpers ---

func optionalCurrency(p PriceModel) *string {
	if !p.PriceCurrency.IsNull() && !p.PriceCurrency.IsUnknown() {
		c := p.PriceCurrency.ValueString()
		return &c
	}
	return nil
}

func buildFixedPriceCreate(p PriceModel) components.ProductPriceFixedCreate {
	return components.ProductPriceFixedCreate{
		PriceAmount:   p.PriceAmount.ValueInt64(),
		PriceCurrency: optionalCurrency(p),
	}
}

func buildCustomPriceCreate(p PriceModel) components.ProductPriceCustomCreate {
	create := components.ProductPriceCustomCreate{
		PriceCurrency: optionalCurrency(p),
	}
	if !p.MinimumAmount.IsNull() {
		v := p.MinimumAmount.ValueInt64()
		create.MinimumAmount = &v
	}
	if !p.MaximumAmount.IsNull() {
		v := p.MaximumAmount.ValueInt64()
		create.MaximumAmount = &v
	}
	if !p.PresetAmount.IsNull() {
		v := p.PresetAmount.ValueInt64()
		create.PresetAmount = &v
	}
	return create
}

func buildMeteredUnitPriceCreate(p PriceModel) components.ProductPriceMeteredUnitCreate {
	create := components.ProductPriceMeteredUnitCreate{
		MeterID:       p.MeterID.ValueString(),
		UnitAmount:    components.CreateUnitAmountStr(p.UnitAmount.ValueString()),
		PriceCurrency: optionalCurrency(p),
	}
	if !p.CapAmount.IsNull() {
		v := p.CapAmount.ValueInt64()
		create.CapAmount = &v
	}
	return create
}

func buildSeatBasedPriceCreate(p PriceModel) components.ProductPriceSeatBasedCreate {
	tiers := make([]components.ProductPriceSeatTier, len(p.SeatTiers))
	for i, t := range p.SeatTiers {
		tier := components.ProductPriceSeatTier{
			MinSeats:     t.MinSeats.ValueInt64(),
			PricePerSeat: t.PricePerSeat.ValueInt64(),
		}
		if !t.MaxSeats.IsNull() {
			v := t.MaxSeats.ValueInt64()
			tier.MaxSeats = &v
		}
		tiers[i] = tier
	}
	return components.ProductPriceSeatBasedCreate{
		PriceCurrency: optionalCurrency(p),
		SeatTiers:     components.ProductPriceSeatTiers{Tiers: tiers},
	}
}

func pricesToRecurringCreateSDK(prices []PriceModel) ([]components.ProductCreateRecurringPrices, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]components.ProductCreateRecurringPrices, len(prices))
	for i, p := range prices {
		switch p.AmountType.ValueString() {
		case "fixed":
			result[i] = components.CreateProductCreateRecurringPricesFixed(buildFixedPriceCreate(p))
		case "free":
			result[i] = components.CreateProductCreateRecurringPricesFree(components.ProductPriceFreeCreate{})
		case "custom":
			result[i] = components.CreateProductCreateRecurringPricesCustom(buildCustomPriceCreate(p))
		case "metered_unit":
			result[i] = components.CreateProductCreateRecurringPricesMeteredUnit(buildMeteredUnitPriceCreate(p))
		case "seat_based":
			result[i] = components.CreateProductCreateRecurringPricesSeatBased(buildSeatBasedPriceCreate(p))
		default:
			diags.AddError("Unsupported price type", "Price amount_type must be one of: fixed, free, custom, metered_unit, seat_based.")
			return nil, diags
		}
	}
	return result, diags
}

func pricesToOneTimeCreateSDK(prices []PriceModel) ([]components.ProductCreateOneTimePrices, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]components.ProductCreateOneTimePrices, len(prices))
	for i, p := range prices {
		switch p.AmountType.ValueString() {
		case "fixed":
			result[i] = components.CreateProductCreateOneTimePricesFixed(buildFixedPriceCreate(p))
		case "free":
			result[i] = components.CreateProductCreateOneTimePricesFree(components.ProductPriceFreeCreate{})
		case "custom":
			result[i] = components.CreateProductCreateOneTimePricesCustom(buildCustomPriceCreate(p))
		case "metered_unit":
			result[i] = components.CreateProductCreateOneTimePricesMeteredUnit(buildMeteredUnitPriceCreate(p))
		case "seat_based":
			result[i] = components.CreateProductCreateOneTimePricesSeatBased(buildSeatBasedPriceCreate(p))
		default:
			diags.AddError("Unsupported price type", "Price amount_type must be one of: fixed, free, custom, metered_unit, seat_based.")
			return nil, diags
		}
	}
	return result, diags
}

// --- Existing price matching for updates ---

// existingPrice holds the identity fields of a current API price for matching.
type existingPrice struct {
	id   string
	data PriceModel // the full model for comparison
	used bool
}

// extractExistingPrices converts the API price response into matchable structs.
func extractExistingPrices(apiPrices []components.Prices) []*existingPrice {
	var result []*existingPrice
	for _, p := range apiPrices {
		if p.ProductPrice == nil {
			continue
		}
		model := sdkProductPriceToModel(p.ProductPrice)
		if model == nil {
			continue
		}
		var id string
		switch {
		case p.ProductPrice.ProductPriceFixed != nil:
			id = p.ProductPrice.ProductPriceFixed.ID
		case p.ProductPrice.ProductPriceFree != nil:
			id = p.ProductPrice.ProductPriceFree.ID
		case p.ProductPrice.ProductPriceCustom != nil:
			id = p.ProductPrice.ProductPriceCustom.ID
		case p.ProductPrice.ProductPriceMeteredUnit != nil:
			id = p.ProductPrice.ProductPriceMeteredUnit.ID
		case p.ProductPrice.ProductPriceSeatBased != nil:
			id = p.ProductPrice.ProductPriceSeatBased.ID
		}
		result = append(result, &existingPrice{id: id, data: *model})
	}
	return result
}

// pricesMatch compares the user-specified fields of two price models.
func pricesMatch(planned, existing PriceModel) bool {
	if planned.AmountType.ValueString() != existing.AmountType.ValueString() {
		return false
	}
	switch planned.AmountType.ValueString() {
	case "fixed":
		if planned.PriceAmount.ValueInt64() != existing.PriceAmount.ValueInt64() {
			return false
		}
		if !planned.PriceCurrency.IsNull() && !planned.PriceCurrency.IsUnknown() &&
			planned.PriceCurrency.ValueString() != existing.PriceCurrency.ValueString() {
			return false
		}
		return true
	case "free":
		return true
	case "custom":
		return optionalInt64Equal(planned.MinimumAmount, existing.MinimumAmount) &&
			optionalInt64Equal(planned.MaximumAmount, existing.MaximumAmount) &&
			optionalInt64Equal(planned.PresetAmount, existing.PresetAmount)
	case "metered_unit":
		if planned.MeterID.ValueString() != existing.MeterID.ValueString() {
			return false
		}
		if !numericStringsEqual(planned.UnitAmount.ValueString(), existing.UnitAmount.ValueString()) {
			return false
		}
		return optionalInt64Equal(planned.CapAmount, existing.CapAmount)
	case "seat_based":
		if len(planned.SeatTiers) != len(existing.SeatTiers) {
			return false
		}
		for j := range planned.SeatTiers {
			pt, et := planned.SeatTiers[j], existing.SeatTiers[j]
			if pt.MinSeats.ValueInt64() != et.MinSeats.ValueInt64() ||
				pt.PricePerSeat.ValueInt64() != et.PricePerSeat.ValueInt64() ||
				!optionalInt64Equal(pt.MaxSeats, et.MaxSeats) {
				return false
			}
		}
		return true
	}
	return false
}

func optionalInt64Equal(a, b types.Int64) bool {
	if a.IsNull() && b.IsNull() {
		return true
	}
	if a.IsNull() != b.IsNull() {
		return false
	}
	return a.ValueInt64() == b.ValueInt64()
}

func pricesToUpdateSDK(planned []PriceModel, currentPrices []components.Prices) ([]components.ProductUpdatePrices, diag.Diagnostics) {
	var diags diag.Diagnostics
	existing := extractExistingPrices(currentPrices)
	result := make([]components.ProductUpdatePrices, len(planned))

	for i, p := range planned {
		// Reuse existing price if values match (avoids unnecessary price recreation)
		var matched bool
		for _, ep := range existing {
			if !ep.used && pricesMatch(p, ep.data) {
				ep.used = true
				result[i] = components.CreateProductUpdatePricesExistingProductPrice(
					components.ExistingProductPrice{ID: ep.id},
				)
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		// Create new price
		switch p.AmountType.ValueString() {
		case "fixed":
			result[i] = components.CreateProductUpdatePricesTwo(
				components.CreateTwoFixed(buildFixedPriceCreate(p)),
			)
		case "free":
			result[i] = components.CreateProductUpdatePricesTwo(
				components.CreateTwoFree(components.ProductPriceFreeCreate{}),
			)
		case "custom":
			result[i] = components.CreateProductUpdatePricesTwo(
				components.CreateTwoCustom(buildCustomPriceCreate(p)),
			)
		case "metered_unit":
			result[i] = components.CreateProductUpdatePricesTwo(
				components.CreateTwoMeteredUnit(buildMeteredUnitPriceCreate(p)),
			)
		case "seat_based":
			result[i] = components.CreateProductUpdatePricesTwo(
				components.CreateTwoSeatBased(buildSeatBasedPriceCreate(p)),
			)
		default:
			diags.AddError("Unsupported price type", "Price amount_type must be one of: fixed, free, custom, metered_unit, seat_based.")
			return nil, diags
		}
	}
	return result, diags
}

// --- Response mapping ---

func sdkPricesToModel(prices []components.Prices) []PriceModel {
	result := make([]PriceModel, 0, len(prices))
	for _, p := range prices {
		if p.ProductPrice != nil {
			model := sdkProductPriceToModel(p.ProductPrice)
			if model != nil {
				result = append(result, *model)
			}
		}
	}
	return result
}

func nullPriceModel(amountType string, currency types.String) PriceModel {
	return PriceModel{
		AmountType:    types.StringValue(amountType),
		PriceCurrency: currency,
		PriceAmount:   types.Int64Null(),
		MinimumAmount: types.Int64Null(),
		MaximumAmount: types.Int64Null(),
		PresetAmount:  types.Int64Null(),
		MeterID:       types.StringNull(),
		UnitAmount:    types.StringNull(),
		CapAmount:     types.Int64Null(),
		SeatTiers:     nil,
	}
}

func sdkProductPriceToModel(price *components.ProductPrice) *PriceModel {
	switch {
	case price.ProductPriceFixed != nil:
		pp := price.ProductPriceFixed
		m := nullPriceModel("fixed", types.StringValue(pp.PriceCurrency))
		m.PriceAmount = types.Int64Value(pp.PriceAmount)
		return &m

	case price.ProductPriceFree != nil:
		m := nullPriceModel("free", types.StringNull())
		return &m

	case price.ProductPriceCustom != nil:
		pp := price.ProductPriceCustom
		m := nullPriceModel("custom", types.StringValue(pp.PriceCurrency))
		m.MinimumAmount = optionalInt64Value(pp.MinimumAmount)
		m.MaximumAmount = optionalInt64Value(pp.MaximumAmount)
		m.PresetAmount = optionalInt64Value(pp.PresetAmount)
		return &m

	case price.ProductPriceMeteredUnit != nil:
		pp := price.ProductPriceMeteredUnit
		m := nullPriceModel("metered_unit", types.StringValue(pp.PriceCurrency))
		m.MeterID = types.StringValue(pp.MeterID)
		m.UnitAmount = types.StringValue(normalizeDecimalString(pp.UnitAmount))
		m.CapAmount = optionalInt64Value(pp.CapAmount)
		return &m

	case price.ProductPriceSeatBased != nil:
		pp := price.ProductPriceSeatBased
		m := nullPriceModel("seat_based", types.StringValue(pp.PriceCurrency))
		tiers := make([]SeatTierModel, len(pp.SeatTiers.Tiers))
		for i, t := range pp.SeatTiers.Tiers {
			tiers[i] = SeatTierModel{
				MinSeats:     types.Int64Value(t.MinSeats),
				MaxSeats:     optionalInt64Value(t.MaxSeats),
				PricePerSeat: types.Int64Value(t.PricePerSeat),
			}
		}
		m.SeatTiers = tiers
		return &m

	default:
		return nil
	}
}

func sdkProductMetadataToMap(ctx context.Context, metadata map[string]components.ProductMetadata, diags *diag.Diagnostics) types.Map {
	if len(metadata) == 0 {
		return types.MapNull(types.StringType)
	}
	return sdkMetadataToMap(ctx, metadata, func(v components.ProductMetadata) metadataFields {
		return metadataFields{Str: v.Str, Integer: v.Integer, Number: v.Number, Boolean: v.Boolean}
	}, diags)
}

// normalizeDecimalString strips trailing zeros from a decimal string so that
// API values like "0.500000000000" become "0.5".
func normalizeDecimalString(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// numericStringsEqual returns true if two numeric strings represent the same
// value (e.g. "0.50" and "0.5" and "0.500000000000" are all equal).
func numericStringsEqual(a, b string) bool {
	af, aErr := strconv.ParseFloat(a, 64)
	bf, bErr := strconv.ParseFloat(b, 64)
	return aErr == nil && bErr == nil && af == bf
}

// preserveUnitAmountFormatting restores the user's original unit_amount
// formatting when the API value is numerically equivalent. This prevents
// Terraform from detecting spurious diffs due to trailing-zero differences
// (e.g. user writes "0.50", API returns "0.500000000000", normalized to "0.5").
func preserveUnitAmountFormatting(prices []PriceModel, priorPrices []PriceModel) {
	for i := range prices {
		if i >= len(priorPrices) {
			break
		}
		if prices[i].UnitAmount.IsNull() || priorPrices[i].UnitAmount.IsNull() {
			continue
		}
		if numericStringsEqual(prices[i].UnitAmount.ValueString(), priorPrices[i].UnitAmount.ValueString()) {
			prices[i].UnitAmount = priorPrices[i].UnitAmount
		}
	}
}
