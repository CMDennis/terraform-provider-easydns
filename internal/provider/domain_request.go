package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// contactModel mirrors the nested contacts schema. Every field is personal
// data; the schema marks the whole block sensitive.
type contactModel struct {
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	OrgName    types.String `tfsdk:"org_name"`
	Address1   types.String `tfsdk:"address1"`
	Address2   types.String `tfsdk:"address2"`
	City       types.String `tfsdk:"city"`
	State      types.String `tfsdk:"state"`
	Country    types.String `tfsdk:"country"`
	PostalCode types.String `tfsdk:"postal_code"`
	Phone      types.String `tfsdk:"phone"`
	Email      types.String `tfsdk:"email"`
	Language   types.String `tfsdk:"language"`
	CPR        types.String `tfsdk:"cpr"`
}

type contactSetModel struct {
	Owner   *contactModel `tfsdk:"owner"`
	Admin   *contactModel `tfsdk:"admin"`
	Tech    *contactModel `tfsdk:"tech"`
	Billing *contactModel `tfsdk:"billing"`
}

func (model *contactModel) toClient() *Contact {
	if model == nil {
		return nil
	}
	return &Contact{
		FirstName:  model.FirstName.ValueString(),
		LastName:   model.LastName.ValueString(),
		OrgName:    model.OrgName.ValueString(),
		Address1:   model.Address1.ValueString(),
		Address2:   model.Address2.ValueString(),
		City:       model.City.ValueString(),
		State:      model.State.ValueString(),
		Country:    model.Country.ValueString(),
		PostalCode: model.PostalCode.ValueString(),
		Phone:      model.Phone.ValueString(),
		Email:      model.Email.ValueString(),
		Language:   model.Language.ValueString(),
		CPR:        model.CPR.ValueString(),
	}
}

// buildDomainRequest converts planned configuration into a client request. It
// does not decide whether registration is allowed; that gate lives in
// ModifyPlan and in the client.
func buildDomainRequest(ctx context.Context, plan DomainResourceModel) (CreateDomainRequest, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	request := CreateDomainRequest{
		Domain:       plan.Domain.ValueString(),
		Service:      DomainService(plan.Service.ValueString()),
		Term:         plan.Term.ValueInt64(),
		Currency:     DomainCurrency(plan.Currency.ValueString()),
		DNSOnly:      plan.DNSOnly.IsNull() || plan.DNSOnly.ValueBool(),
		Premium:      !plan.Premium.IsNull() && plan.Premium.ValueBool(),
		PremiumPrice: plan.PremiumPrice.ValueString(),
		DomainGroup:  plan.DomainGroup.ValueString(),
		PrimaryNS:    plan.PrimaryNS.ValueString(),
	}

	if !plan.Nameservers.IsNull() && !plan.Nameservers.IsUnknown() {
		var nameservers []string
		diagnostics.Append(plan.Nameservers.ElementsAs(ctx, &nameservers, false)...)
		request.Nameservers = nameservers
	}

	if !plan.Extra.IsNull() && !plan.Extra.IsUnknown() {
		extra := map[string]string{}
		diagnostics.Append(plan.Extra.ElementsAs(ctx, &extra, false)...)
		request.Extra = extra
	}

	if !plan.Contacts.IsNull() && !plan.Contacts.IsUnknown() {
		var contacts contactSetModel
		diagnostics.Append(plan.Contacts.As(ctx, &contacts, basetypes.ObjectAsOptions{})...)
		if !diagnostics.HasError() {
			set := &ContactSet{
				Owner:   contacts.Owner.toClient(),
				Admin:   contacts.Admin.toClient(),
				Tech:    contacts.Tech.toClient(),
				Billing: contacts.Billing.toClient(),
			}
			if set.Owner == nil {
				diagnostics.AddAttributeError(path.Root("contacts").AtName("owner"),
					"Registration Requires An Owner Contact",
					"contacts must include an owner block to register a domain.")
			}
			request.Contacts = set
		}
	}

	// The premium ceiling is re-checked here so a value that reached the plan
	// through an unknown reference cannot slip past config validation.
	if request.Premium && plan.PremiumPrice.ValueString() != "" && !plan.MaxPremiumPrice.IsNull() {
		if err := assertPremiumPriceWithinCeiling(plan.PremiumPrice.ValueString(), plan.MaxPremiumPrice.ValueString()); err != nil {
			diagnostics.AddAttributeError(path.Root("premium_price"), "Premium Price Exceeds The Accepted Maximum", err.Error())
		}
	}

	return request, diagnostics
}
