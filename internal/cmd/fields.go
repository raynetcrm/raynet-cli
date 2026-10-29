/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

// Field reference for create/update help, extracted from the REST API's
// OpenAPI spec (create endpoint request bodies). Required first, then by
// usefulness; rarely used, custom and deprecated fields are left out.

var (
	priorityValues = []string{"MINOR", "DEFAULT", "CRITICAL"}

	ownerField = field{name: "owner", kind: kindID, ref: "user", help: "owner (user's person id)"}
	tagsField  = field{name: "tags", kind: kindArray, help: "tag names, e.g. [\"vip\",\"2026\"]"}
)

var companyFields = []field{
	{name: "name", kind: kindString, example: "Acme Ltd", required: true, help: "company name"},
	{name: "state", kind: kindString, required: true, values: []string{"A_POTENTIAL", "B_ACTUAL", "C_DEFERRED", "D_UNATTRACTIVE"}, help: "lifecycle state"},
	{name: "rating", kind: kindString, required: true, values: []string{"A", "B", "C"}, help: "rating"},
	{name: "role", kind: kindString, required: true, values: []string{"A_SUBSCRIBER", "B_PARTNER", "C_SUPPLIER", "D_RIVAL"}, help: "relationship to you"},
	ownerField,
	{name: "addresses", kind: kindArray, help: "[{address:{name,street,city,zipCode,country}}]"},
	{name: "regNumber", kind: kindString, example: "12345678", help: "company registration number (ICO)"},
	{name: "taxNumber", kind: kindString, help: "VAT number (DIC)"},
	{name: "category", kind: kindID, codelist: "companyCategory", help: "category"},
	{name: "contactSource", kind: kindID, codelist: "contactSource", help: "contact source"},
	{name: "economyActivity", kind: kindID, codelist: "economyActivity", help: "industry"},
	{name: "legalForm", kind: kindID, codelist: "legalForm", help: "legal form"},
	{name: "paymentTerm", kind: kindID, codelist: "paymentTerm", help: "payment terms"},
	{name: "person", kind: kindBool, help: "true if the client is a natural person"},
	{name: "firstName", kind: kindString, help: "first name (natural person)"},
	{name: "lastName", kind: kindString, help: "last name, required when person is true"},
	{name: "taxPayer", kind: kindString, values: []string{"YES", "NO"}, help: "VAT payer"},
	{name: "bankAccount", kind: kindString, help: "bank account"},
	{name: "birthday", kind: kindDate, help: "birthday or anniversary, yyyy-MM-dd"},
	{name: "notice", kind: kindString, help: "note"},
	tagsField,
}

var personFields = []field{
	{name: "lastName", kind: kindString, example: "Novak", required: true, help: "last name"},
	{name: "firstName", kind: kindString, help: "first name"},
	{name: "titleBefore", kind: kindString, help: "title before the name"},
	{name: "titleAfter", kind: kindString, help: "title after the name"},
	ownerField,
	{name: "contactInfo", kind: kindObject, help: "{email, email2, tel1, tel1Type, tel2, www}"},
	{name: "relationship", kind: kindObject, help: "primary employer: {company, type (position), notice}"},
	{name: "category", kind: kindID, codelist: "personCategory", help: "category"},
	{name: "salutation", kind: kindString, help: "salutation"},
	{name: "gender", kind: kindString, values: []string{"MALE", "FEMALE"}, help: "gender"},
	{name: "birthday", kind: kindDate, help: "birthday, yyyy-MM-dd"},
	{name: "language", kind: kindID, codelist: "language", help: "language"},
	{name: "privateAddress", kind: kindObject, help: "{street, city, province, zipCode, country}"},
	{name: "keyman", kind: kindBool, help: "key person"},
	{name: "notice", kind: kindString, help: "note"},
	tagsField,
}

var leadFields = []field{
	{name: "topic", kind: kindString, example: "Website enquiry", required: true, help: "subject"},
	{name: "priority", kind: kindString, required: true, values: priorityValues, help: "priority"},
	ownerField,
	{name: "companyName", kind: kindString, help: "company name"},
	{name: "firstName", kind: kindString, help: "contact first name"},
	{name: "lastName", kind: kindString, help: "contact last name"},
	{name: "contactInfo", kind: kindObject, help: "{email, tel1, tel1Type, www}"},
	{name: "address", kind: kindObject, help: "{street, city, province, zipCode, country}"},
	{name: "regNumber", kind: kindString, example: "12345678", help: "company registration number (ICO)"},
	{name: "leadPhase", kind: kindID, codelist: "leadPhase", help: "lead phase; defaults to the first phase"},
	{name: "category", kind: kindID, codelist: "leadCategory", help: "category"},
	{name: "contactSource", kind: kindID, codelist: "contactSource", help: "contact source"},
	{name: "territory", kind: kindID, codelist: "territory", help: "sales territory"},
	{name: "leadDate", kind: kindDate, help: "date the lead was received, yyyy-MM-dd"},
	{name: "leadPerson", kind: kindBool, help: "true if a natural person, false if a company"},
	{name: "notice", kind: kindString, help: "note"},
	{name: "tags", kind: kindString, help: "comma-separated tag names"},
}

var businessCaseFields = []field{
	{name: "name", kind: kindString, example: "CRM rollout", required: true, help: "subject"},
	{name: "company", kind: kindID, required: true, ref: "company", help: "client"},
	ownerField,
	{name: "person", kind: kindID, ref: "person", help: "client's contact person"},
	{name: "businessCasePhase", kind: kindID, codelist: "businessCasePhase", help: "phase; defaults to the first phase"},
	{name: "totalAmount", kind: kindNumber, help: "total price"},
	{name: "estimatedValue", kind: kindNumber, help: "estimated costs"},
	{name: "probability", kind: kindNumber, help: "win probability, percent"},
	{name: "validFrom", kind: kindDate, help: "opened on, yyyy-MM-dd"},
	{name: "validTill", kind: kindDate, only: verbUpdate, help: "closing date, yyyy-MM-dd"},
	{name: "currency", kind: kindID, codelist: "currency", help: "currency"},
	{name: "category", kind: kindID, codelist: "businessCaseCategory", help: "category"},
	{name: "source", kind: kindID, codelist: "contactSource", help: "contact source"},
	{name: "project", kind: kindID, ref: "project", help: "project the business case belongs to"},
	{name: "items", kind: kindArray, help: "line items, see `item add --help`"},
	{name: "description", kind: kindString, help: "note"},
	tagsField,
}

var offerFields = []field{
	{name: "name", kind: kindString, example: "CRM rollout", required: true, help: "subject"},
	{name: "company", kind: kindID, required: true, ref: "company", help: "client"},
	{name: "businessCase", kind: kindID, required: true, ref: "business-case", help: "business case"},
	ownerField,
	{name: "person", kind: kindID, ref: "person", help: "client's contact person"},
	{name: "offerStatus", kind: kindID, codelist: "offerStatus", help: "status; defaults to the first status"},
	{name: "totalAmount", kind: kindNumber, help: "total price"},
	{name: "estimatedValue", kind: kindNumber, help: "estimated costs"},
	{name: "validFrom", kind: kindDate, help: "created on, yyyy-MM-dd"},
	{name: "validTill", kind: kindDate, help: "closed on, yyyy-MM-dd"},
	{name: "expirationDate", kind: kindDate, help: "valid until, yyyy-MM-dd"},
	{name: "category", kind: kindID, codelist: "offerCategory", help: "category"},
	{name: "sync", kind: kindBool, only: verbCreate, help: "copy items from the business case"},
	{name: "items", kind: kindArray, help: "line items, see `item add --help`"},
	{name: "description", kind: kindString, help: "note"},
}

var salesOrderFields = []field{
	{name: "name", kind: kindString, example: "CRM licences", required: true, help: "subject"},
	{name: "company", kind: kindID, required: true, ref: "company", help: "client"},
	{name: "businessCase", kind: kindID, required: true, ref: "business-case", help: "business case"},
	ownerField,
	{name: "person", kind: kindID, ref: "person", help: "client's contact person"},
	{name: "offer", kind: kindID, ref: "offer", help: "offer the order is based on"},
	{name: "salesOrderStatus", kind: kindID, codelist: "salesOrderStatus", help: "status; defaults to the first status"},
	{name: "totalAmount", kind: kindNumber, help: "total price"},
	{name: "estimatedValue", kind: kindNumber, help: "estimated costs"},
	{name: "validFrom", kind: kindDate, help: "created on, yyyy-MM-dd"},
	{name: "validTill", kind: kindDate, help: "closed on, yyyy-MM-dd"},
	{name: "expirationDate", kind: kindDate, help: "valid until, yyyy-MM-dd"},
	{name: "requestDeliveryDate", kind: kindDate, help: "deliver by, yyyy-MM-dd"},
	{name: "category", kind: kindID, codelist: "salesOrderCategory", help: "category"},
	{name: "deliveryAddress", kind: kindObject, help: "{name, street, city, zipCode, countryCode}; country instead of countryCode on update and with items"},
	{name: "invoiceAddress", kind: kindObject, help: "{name, street, city, zipCode, countryCode}; country instead of countryCode on update and with items"},
	{name: "sync", kind: kindBool, only: verbCreate, help: "copy items from the business case"},
	{name: "items", kind: kindArray, help: "line items, see `item add --help`"},
	{name: "description", kind: kindString, help: "note"},
}

var projectFields = []field{
	{name: "name", kind: kindString, example: "CRM rollout", required: true, help: "subject"},
	{name: "company", kind: kindID, required: true, ref: "company", help: "client"},
	ownerField,
	{name: "person", kind: kindID, ref: "person", help: "client's contact person"},
	{name: "projectStatus", kind: kindID, codelist: "projectStatus", help: "status; defaults to the first status"},
	{name: "totalAmount", kind: kindNumber, help: "total price"},
	{name: "validFrom", kind: kindDate, help: "opened on, yyyy-MM-dd"},
	{name: "validTill", kind: kindDate, help: "closed on, yyyy-MM-dd"},
	{name: "scheduledEnd", kind: kindDate, help: "planned end, yyyy-MM-dd"},
	{name: "category", kind: kindID, codelist: "projectCategory", help: "category"},
	{name: "description", kind: kindString, help: "note"},
	tagsField,
}

var invoiceFields = []field{
	{name: "code", kind: kindString, example: "2026-0042", required: true, help: "invoice number"},
	{name: "company", kind: kindID, required: true, ref: "company", help: "client being invoiced"},
	{name: "currency", kind: kindID, required: true, codelist: "currency", help: "currency"},
	{name: "issueDate", kind: kindDate, required: true, help: "issue date, yyyy-MM-dd"},
	{name: "dueDate", kind: kindDate, required: true, help: "due date, yyyy-MM-dd"},
	{name: "taxableSupplyDate", kind: kindDate, required: true, help: "taxable supply date, yyyy-MM-dd"},
	{name: "invoiceType", kind: kindString, required: true, values: []string{"NORMAL", "PROFORMA"}, help: "invoice type"},
	{name: "invoiceState", kind: kindString, required: true, values: []string{"UNPAID", "PARTIALLY_PAID", "PAID", "CANCELLED"}, help: "payment state"},
	{name: "paymentType", kind: kindString, required: true, values: []string{"PAYMENT_DIRECT_DEBIT", "PAYMENT_CASH", "PAYMENT_CASH_ON_DELIVERY", "PAYMENT_CARD"}, help: "payment method"},
	{name: "taxPayer", kind: kindString, required: true, values: []string{"YES", "NO"}, help: "vendor is a VAT payer"},
	{name: "billingName", kind: kindString, example: "Acme Ltd", required: true, help: "customer name"},
	{name: "billingAddress", kind: kindObject, required: true, help: "{street, city, province, zipCode, country}"},
	{name: "title", kind: kindString, help: "invoice title, for searching"},
	ownerField,
	{name: "items", kind: kindArray, help: "[{name, unitPrice, amount, taxRate, unitLabel}]"},
	{name: "businessCase", kind: kindID, ref: "business-case", help: "related business case"},
	{name: "salesOrder", kind: kindID, ref: "sales-order", help: "related sales order"},
	{name: "variableSymbol", kind: kindString, help: "variable symbol (payment reference)"},
	{name: "billingRegNumber", kind: kindString, help: "customer registration number (ICO)"},
	{name: "billingTaxNumber", kind: kindString, help: "customer VAT number (DIC)"},
	{name: "paymentDate", kind: kindDate, help: "paid on, yyyy-MM-dd"},
	{name: "category", kind: kindID, codelist: "invoiceCategory", help: "category"},
	{name: "note", kind: kindString, help: "note for the recipient"},
	{name: "privateNote", kind: kindString, help: "internal note"},
	tagsField,
}

var productFields = []field{
	{name: "code", kind: kindString, example: "CRM-PRO", required: true, help: "product code"},
	{name: "name", kind: kindString, example: "CRM Professional", required: true, help: "product name"},
	{name: "price", kind: kindNumber, help: "standard price"},
	{name: "cost", kind: kindNumber, help: "cost"},
	{name: "taxRate", kind: kindNumber, help: "VAT rate, percent"},
	{name: "unit", kind: kindString, help: "unit, e.g. pcs"},
	{name: "category", kind: kindID, codelist: "productCategory", help: "category"},
	{name: "productLine", kind: kindID, codelist: "productLine", help: "product line"},
	{name: "description", kind: kindString, help: "description"},
	tagsField,
}

var priceListFields = []field{
	{name: "name", kind: kindString, example: "Standard 2026", required: true, help: "price list name"},
	{name: "code", kind: kindString, example: "STD-2026", required: true, help: "price list code"},
	{name: "currency", kind: kindID, required: true, codelist: "currency", help: "currency"},
	{name: "validFrom", kind: kindDate, required: true, help: "valid from, yyyy-MM-dd"},
	{name: "validTill", kind: kindDate, help: "valid until, yyyy-MM-dd"},
	ownerField,
	{name: "category", kind: kindID, codelist: "priceListCategory", help: "category"},
	{name: "description", kind: kindString, help: "note"},
}

// Activity bodies differ between create and update: owner, resolver and lead
// are create-only, status, personal and participants update-only, and tags
// is an array on create but a comma-separated string on update.
var (
	activityStatusValues = []string{"NEW", "SCHEDULED", "COMPLETED", "CANCELLED"}

	activityTagsCreateField = field{name: "tags", kind: kindArray, only: verbCreate, help: "tag names, e.g. [\"vip\",\"2026\"]"}
	activityTagsUpdateField = field{name: "tags", kind: kindString, only: verbUpdate, help: "comma-separated tag names"}
)

// activityFields builds the field list of an activity type: the shared
// fields, with typeFields after the required ones. The contact person is
// create-only except on task, whose update body takes it too.
func activityFields(personOnly verb, descriptionHelp string, typeFields ...field) []field {
	return concatFields(
		[]field{
			{name: "title", kind: kindString, example: "Follow-up call", required: true, help: "subject"},
			{name: "priority", kind: kindString, required: true, values: priorityValues, help: "priority"},
			{name: "owner", kind: kindID, required: true, ref: "user", only: verbCreate, help: "owner (user's person id); update changes it through participants"},
		},
		typeFields,
		[]field{
			{name: "status", kind: kindString, values: activityStatusValues, only: verbUpdate, help: "status"},
			{name: "company", kind: kindID, ref: "company", help: "related client"},
			{name: "person", kind: kindID, ref: "person", only: personOnly, help: "related contact person"},
			{name: "businessCase", kind: kindID, ref: "business-case", help: "related business case"},
			{name: "offer", kind: kindID, ref: "offer", help: "related offer"},
			{name: "salesOrder", kind: kindID, ref: "sales-order", help: "related sales order"},
			{name: "project", kind: kindID, ref: "project", help: "related project"},
			{name: "lead", kind: kindID, ref: "lead", only: verbCreate, help: "related lead"},
			{name: "category", kind: kindID, codelist: "activityCategory", help: "category"},
			{name: "scheduledFrom", kind: kindDateTime, help: "calendar start, yyyy-MM-dd HH:mm"},
			{name: "scheduledTill", kind: kindDateTime, help: "calendar end, yyyy-MM-dd HH:mm"},
			{name: "completed", kind: kindDateTime, help: "completed at, yyyy-MM-dd HH:mm"},
			{name: "description", kind: kindString, help: descriptionHelp},
			{name: "personal", kind: kindBool, only: verbUpdate, help: "private activity"},
			{name: "participants", kind: kindArray, only: verbUpdate, help: "[{id, person, company, lead, owner, role}]; no id adds, a negative id removes"},
			activityTagsCreateField,
			activityTagsUpdateField,
		},
	)
}

func concatFields(parts ...[]field) []field {
	var out []field
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var (
	taskFields = activityFields(0, "assignment",
		field{name: "deadline", kind: kindDateTime, required: true, help: "deadline, yyyy-MM-dd HH:mm"},
		field{name: "resolver", kind: kindID, ref: "user", only: verbCreate, help: "assignee (user's person id); defaults to owner"},
		field{name: "solution", kind: kindString, help: "solution"},
	)
	meetingFields = activityFields(verbCreate, "topics to discuss",
		field{name: "solution", kind: kindString, help: "meeting outcome"},
		field{name: "meetingPlace", kind: kindString, only: verbUpdate, help: "place"},
	)
	eventFields = activityFields(verbCreate, "event details",
		field{name: "meetingPlace", kind: kindString, only: verbUpdate, help: "place"},
	)
	emailFields     = activityFields(verbCreate, "email body")
	phoneCallFields = activityFields(verbCreate, "topics to discuss",
		field{name: "solution", kind: kindString, help: "call outcome"},
	)
	letterFields = activityFields(verbCreate, "letter body")
)

var itemFields = []field{
	{name: "name", kind: kindString, example: "CRM Professional", required: true, help: "item name; may be omitted when the item links a product"},
	{name: "count", kind: kindNumber, help: "quantity, default 1"},
	{name: "price", kind: kindNumber, help: "unit selling price"},
	{name: "taxRate", kind: kindNumber, help: "VAT rate, percent"},
	{name: "discountPercent", kind: kindNumber, help: "discount, percent"},
	{name: "product", kind: kindID, ref: "product", only: verbCreate, help: "product to link the item to"},
	{name: "productCode", kind: kindString, only: verbCreate, help: "product code to look the product up by and link it"},
	{name: "priceList", kind: kindID, ref: "price-list", only: verbCreate, help: "price list to look the product up in"},
	{name: "priceListItem", kind: kindID, help: "price list item to link; on update offer and sales order only"},
	{name: "cost", kind: kindNumber, help: "cost per unit"},
	{name: "unit", kind: kindString, help: "unit, e.g. pcs"},
	{name: "description", kind: kindString, help: "note"},
}

var companyAddressFields = []field{
	{name: "address", kind: kindObject, required: true, help: "{name (required), street, city, province, zipCode, country (ISO code, e.g. CZ), lat, lng}, e.g. -f 'address[name]=HQ'"},
	{name: "contactInfo", kind: kindObject, help: "{email, email2, tel1, tel1Type, tel2, tel2Type, www, fax, otherContact, doNotSendMM}"},
	{name: "territory", kind: kindID, codelist: "territory", help: "sales territory"},
}

var companyRelationshipFields = []field{
	{name: "companyJoin", kind: kindID, required: true, ref: "company", only: verbCreate, help: "the other client"},
	{name: "companyJoinType", kind: kindString, required: true, values: []string{"parent", "slave", "free"}, only: verbCreate,
		help: "this client is the parent of / a subsidiary of / loosely linked to companyJoin"},
	{name: "notice", kind: kindString, help: "note; the only field update changes"},
}

var personRelationshipFields = []field{
	{name: "company", kind: kindID, required: true, ref: "company", help: "client (employer)"},
	{name: "type", kind: kindString, required: true, help: "position, e.g. CEO"},
	{name: "companyAddress", kind: kindID, help: "id of one of the client's addresses"},
	{name: "notice", kind: kindString, help: "note"},
}

var leadConvertFields = []field{
	{name: "company", kind: kindID, ref: "company", help: "existing client to convert into"},
	{name: "person", kind: kindID, ref: "person", help: "existing contact person to convert into"},
	{name: "businessCase", kind: kindID, ref: "business-case", help: "existing business case; overrides company/person (send at least one field)"},
}
