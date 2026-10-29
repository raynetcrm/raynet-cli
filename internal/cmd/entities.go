/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"net/http"

	"github.com/spf13/cobra"
)

// entityTable is the entity spec table: one row per entity command, turned
// into its command subtree by newEntityCmd. Paths, columns and filters are
// checked by hand against the REST API's OpenAPI spec.
var entityTable = []entitySpec{
	{
		use:         "company",
		short:       "Work with companies (clients)",
		path:        "company",
		webName:     "Company",
		columns:     []string{"id", "name", "regNumber", "state", "rating", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "name[LIKE], regNumber, state, rating, role, category, owner, tags, primaryAddress-address.city",
		fields:      companyFields,
		nested:      []nestedSpec{companyAddresses, companyRelationships},
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actTag, actUntag},
	},
	{
		use:         "person",
		short:       "Work with persons (contacts)",
		path:        "person",
		webName:     "Person",
		columns:     []string{"id", "firstName", "lastName", "primaryRelationship.company.name", "contactInfo.email", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "lastName[LIKE], firstName, contactInfo.email, primaryRelationship-company-id, category, owner, tags",
		fields:      personFields,
		nested:      []nestedSpec{personRelationships},
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actTag, actUntag},
	},
	// Business entities (#8).
	{
		use:         "lead",
		short:       "Work with leads",
		path:        "lead",
		webName:     "Lead",
		columns:     []string{"id", "code", "topic", "companyName", "leadPhase.code01", "priority", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "topic[LIKE], companyName[LIKE], lastName, contactInfo.email, regNumber, status (B_ACTIVE, G_STORNO, D_DONE), leadPhase, leadDate, owner, tags",
		fields:      leadFields,
		actions: []action{actLock, actUnlock, {
			use: "convert", short: "Convert %s by linking it to an existing client, contact person or business case",
			done: "Converted", method: http.MethodPost, suffix: "convert/", body: true, fields: leadConvertFields,
		}, actTag, actUntag},
	},
	{
		use:         "business-case",
		aliases:     []string{"businessCase", "bc"},
		short:       "Work with business cases",
		path:        "businessCase",
		webName:     "BusinessCase",
		columns:     []string{"id", "code", "name", "company.name", "businessCasePhase.value", "totalAmount", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		withItems:   true,
		listFilters: "name[LIKE], code, company, project, status (B_ACTIVE, E_WIN, F_LOST, G_STORNO), businessCasePhase, category, validFrom, validTill, owner",
		fields:      businessCaseFields,
		nested:      []nestedSpec{lineItems},
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actTag, actUntag},
	},
	{
		use:         "offer",
		short:       "Work with offers",
		path:        "offer",
		webName:     "Offer",
		columns:     []string{"id", "code", "name", "company.name", "offerStatus.value", "totalAmount", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		withItems:   true,
		listFilters: "name[LIKE], code, company, person, businessCase, status (B_ACTIVE, E_WIN, F_LOST, G_STORNO), offerStatus, category, expirationDate, owner",
		fields:      offerFields,
		nested:      []nestedSpec{lineItems},
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actSync, actUnsync, actTag, actUntag},
	},
	{
		use:         "sales-order",
		aliases:     []string{"salesOrder", "so"},
		short:       "Work with sales orders",
		path:        "salesOrder",
		webName:     "SalesOrder",
		columns:     []string{"id", "code", "name", "company.name", "salesOrderStatus.value", "totalAmount", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		withItems:   true,
		listFilters: "name[LIKE], code, company, person, businessCase, status (B_ACTIVE, E_WIN, F_LOST, G_STORNO), salesOrderStatus, category, requestDeliveryDate, owner",
		fields:      salesOrderFields,
		nested:      []nestedSpec{lineItems},
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actSync, actUnsync, actTag, actUntag},
	},
	{
		use:         "project",
		short:       "Work with projects",
		path:        "project",
		webName:     "Project",
		columns:     []string{"id", "code", "name", "company.name", "projectStatus.value", "owner.fullName", "scheduledEnd"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "name[LIKE], code, company, person, projectStatus, category, validFrom, validTill, owner, tags",
		fields:      projectFields,
		actions:     []action{actLock, actUnlock, actInvalidate, actRevalidate, actTag, actUntag},
	},
	{
		use:         "invoice",
		aliases:     []string{"invoiceLight"},
		short:       "Work with invoices",
		path:        "invoiceLight",
		webName:     "Invoice",
		columns:     []string{"id", "code", "title", "company.name", "invoiceType", "invoiceState", "totalAmount", "dueDate"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "title[LIKE], code, company, businessCase, invoiceType (NORMAL, PROFORMA, CREDIT_NOTE), issueDate, dueDate, variableSymbol, owner, tags",
		fields:      invoiceFields,
		actions:     []action{actLock, actUnlock, actTag, actUntag},
	},
	{
		use:         "product",
		short:       "Work with products in the catalog",
		path:        "product",
		webName:     "Product",
		columns:     []string{"id", "code", "name", "primaryPriceListItem.price", "unit", "productLine.value", "category.value"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "name[LIKE], category, productLine, unit, rowInfo.rowAccess (INVALID), tags",
		fields:      productFields,
		actions:     []action{actInvalidate, actRevalidate, actTag, actUntag},
	},
	{
		use:         "price-list",
		aliases:     []string{"priceList", "pl"},
		short:       "Work with price lists",
		path:        "priceList",
		webName:     "PriceList",
		columns:     []string{"id", "code", "name", "currency.value", "primary", "validFrom", "validTill", "owner.fullName"},
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: "code[LIKE], primary, currency, validFrom, validTill",
		fields:      priceListFields,
		actions:     []action{actLock, actUnlock, actTag, actUntag},
	},
	{
		use:         "user-account",
		aliases:     []string{"userAccount"},
		short:       "Look up CRM users; owner fields take a user's person.id",
		path:        "userAccount",
		columns:     []string{"id", "username", "person.fullName", "person.id", "userRole"},
		verbs:       verbList | verbView,
		listFilters: "username[LIKE], userRole (ADMIN, FULL_THROTTLE, CERTAIN_RESTRICTIONS, READONLY)",
		commands:    func(a *app) []*cobra.Command { return []*cobra.Command{newUserMeCmd(a)} },
	},

	// Activities: one row per type, plus the list-only activity row.
	activityRow("task", nil, "Work with tasks", "task", "Task",
		[]string{"id", "title", "status", "priority", "deadline", "company.name", "person.name"},
		activityListFilters+", deadline[LT]", taskFields),
	activityRow("meeting", nil, "Work with meetings", "meeting", "Meeting",
		activityColumns, activityListFilters, meetingFields),
	activityRow("event", nil, "Work with events", "event", "Event",
		activityColumns, activityListFilters, eventFields),
	activityRow("email", nil, "Work with emails", "email", "Email",
		activityColumns, activityListFilters, emailFields),
	activityRow("phone-call", []string{"phoneCall", "pc"}, "Work with phone calls", "phoneCall", "PhoneCall",
		activityColumns, activityListFilters, phoneCallFields),
	activityRow("letter", nil, "Work with letters", "letter", "Letter",
		activityColumns, activityListFilters, letterFields),
	{
		use:         "activity",
		short:       "List all activity types together; act on one through its type's command",
		path:        "activity",
		webName:     "Activity",
		columns:     []string{"id", "type", "title", "status", "priority", "scheduledFrom", "company.name"},
		verbs:       verbList,
		listFilters: activityListFilters,
		typeColumn:  true,
	},
}

// activityColumns are the default list columns of the non-task activity
// types; activity list rows carry no owner.
var activityColumns = []string{"id", "title", "status", "priority", "scheduledFrom", "company.name", "person.name"}

const activityListFilters = "title[LIKE], status, scheduledFrom[GE], owner-id, category-id, companyContextFilter, tags"

// activityRow is the row of one activity type: full CRUD, ext lookup, and
// lock/unlock/tag/untag on the type's own path (there is no
// /activity/<id>/lock).
func activityRow(use string, aliases []string, short, path, webName string, columns []string, listFilters string, fields []field) entitySpec {
	return entitySpec{
		use:         use,
		aliases:     aliases,
		short:       short,
		path:        path,
		webName:     webName,
		columns:     columns,
		verbs:       verbsCRUD,
		ext:         true,
		listFilters: listFilters,
		fields:      fields,
		actions:     []action{actLock, actUnlock, actTag, actUntag},
	}
}

// lineItems is the line item collection of a business document. The API
// has no item list endpoint: items come from the document's view response.
var lineItems = nestedSpec{
	use:        "item",
	plural:     "items",
	short:      "Work with a business document's line items",
	segment:    "item",
	viewField:  "items",
	columns:    []string{"id", "name", "priceListItem.product.code", "count", "unit", "price", "discountPercent", "taxRate"},
	verbs:      verbList | verbCreate | verbUpdate | verbDelete,
	bulkDelete: true,
	fields:     itemFields,
}

// companyAddresses is a company's address collection. The API has no
// address list endpoint: addresses come from the company's view response.
var companyAddresses = nestedSpec{
	use:       "address",
	plural:    "addresses",
	short:     "Work with a company's addresses",
	segment:   "address",
	viewField: "addresses",
	columns:   []string{"id", "primary", "contactAddress", "address.name", "address.street", "address.city", "address.zipCode", "address.country", "contactInfo.email", "contactInfo.tel1"},
	verbs:     verbList | verbCreate | verbUpdate | verbDelete,
	fields:    companyAddressFields,
	actions: []action{
		{use: "set-primary", short: "Make an address the primary address of %s; the previous one becomes an other address",
			done: "Set primary", method: http.MethodPost, suffix: "setPrimary/"},
		{use: "set-contact", short: "Make an address the contact address of %s, shown in lists and first in detail",
			done: "Set contact", method: http.MethodPost, suffix: "setContact/"},
	},
}

// companyRelationships links a company to other companies; adding or
// deleting one also adds or deletes the inverse link on the other company.
var companyRelationships = nestedSpec{
	use:     "relationship",
	plural:  "relationships",
	short:   "Work with a company's links to other companies (parent, subsidiary, loose)",
	segment: "relationship",
	columns: []string{"id", "companyJoin.id", "companyJoin.name", "companyJoinType", "notice"},
	verbs:   verbList | verbCreate | verbUpdate | verbDelete,
	fields:  companyRelationshipFields,
}

// personRelationships are the companies a person belongs to (job
// positions). The person's view response carries them, but the spec offers
// no list: `person view` shows them.
var personRelationships = nestedSpec{
	use:     "relationship",
	plural:  "relationships",
	short:   "Work with the companies a person belongs to (job positions)",
	segment: "relationship",
	verbs:   verbCreate | verbUpdate | verbDelete,
	// The API requires company and type on update as well.
	updateRequired: true,
	fields:         personRelationshipFields,
	actions: []action{
		{use: "set-primary", short: "Make a relationship the primary company of %s; the previous one becomes secondary",
			done: "Set primary", method: http.MethodPost, suffix: "setPrimary/"},
	},
}
